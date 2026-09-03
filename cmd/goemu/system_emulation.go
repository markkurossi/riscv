//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package main

import (
	"bytes"
	"debug/elf"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/markkurossi/gofdt"
	"github.com/markkurossi/riscv/cpu"
	"github.com/markkurossi/riscv/dev"
	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/kernel"
	"github.com/markkurossi/riscv/memory"
	"github.com/markkurossi/riscv/mmu"
	"github.com/markkurossi/riscv/virtio"
)

const (
	OfsBIOS   = 0x8000_0000
	OfsKernel = 0x8020_0000
	OfsDTB    = 0x8220_0000
	OfsInitrd = 0x8800_0000

	PLICBase = 0x0c00_0000

	CLINTBase = 0x200_0000
	CLINTSize = 0x10000

	UARTBase = 0x1000_0000
	UARTIRQ  = 10

	SysconBase = 0x1000_0100
	SysconSize = 256

	NMEABase = 0x1000_0200
	NMEAIRQ  = 11

	VirtioRAMBase = 0x1000_8000
	VirtioIRQBase = 1

	RTCBase = 0x1010_0000
	RTCSize = 0x1000
	RTCIRQ  = 12

	SHMBase = 0x4000_0000
)

func systemEmulation(htif bool, params kernel.Params, cfg *SystemConfig,
	args []string) error {

	ramSize, err := ParseMem(cfg.Memory)
	if err != nil {
		return err
	}
	mem := memory.New(memory.RAMBase, ramSize)

	hart := cpu.New(mem)
	hart.Trace = params.CPUtrace
	hart.CSR802Filename = params.CSR802

	plic := dev.NewPLIC([]isa.Hart{hart}, PLICBase)

	var uarts []*dev.UART

	uart := dev.NewUART(hart, "Cons", UARTBase, plic, UARTIRQ, &dev.UARTConsole{
		Hart:   hart,
		Color:  params.Color,
		Cooked: params.Cooked,
	})
	uarts = append(uarts, uart)

	nmea := dev.NewUART(hart, "NMEA", NMEABase, plic, NMEAIRQ, &dev.UARTNMEA{
		Hart: hart,
	})
	uarts = append(uarts, nmea)

	mmio := &MMIO{
		Hart: hart,
		Segments: []mmu.MMIO{
			plic,
			uart,
			nmea,
			dev.NewSyscon(hart, SysconBase, SysconSize),
			&dev.GoldfishRTC{
				Hart:  hart,
				Start: RTCBase,
				End:   RTCBase + RTCSize,
			},
			&dev.CLINT{
				Hart:  hart,
				Start: CLINTBase,
				End:   CLINTBase + CLINTSize,
			},
		},
	}

	// Create VirtIO devices.
	var virtioDevices []*virtio.MMIO

	var virtioRAM uint64 = VirtioRAMBase
	var virtioIRQ uint32 = VirtioIRQBase

	var shmBase uint64 = SHMBase

	// Entropy Source / RNG.
	rng := virtio.NewRng(hart, virtioRAM, plic, virtioIRQ, mem)
	mmio.Segments = append(mmio.Segments, rng)
	virtioRAM = rng.End
	virtioIRQ++
	virtioDevices = append(virtioDevices, rng.Device())

	var gpu *virtio.GPU

	// Devices from the configuration.
	for idx, dev := range cfg.Devices {
		var vio *virtio.MMIO

		switch dev.Type {
		case "virtio-blk-device":
			drive := cfg.Drive(dev.Drive)
			if drive == nil {
				return fmt.Errorf("unknown drive: %v", dev.Drive)
			}
			var flags int
			if drive.Readonly {
				flags = os.O_RDONLY
			} else {
				flags = os.O_RDWR
			}
			fs, err := os.OpenFile(drive.File, flags, 0644)
			if err != nil {
				return err
			}

			blk := virtio.NewBlk(hart, virtioRAM, plic, virtioIRQ, mem, fs)
			mmio.Segments = append(mmio.Segments, blk)

			deviceID := dev.ID
			if len(deviceID) == 0 {
				deviceID = fmt.Sprintf("goemu-disk-%d", idx)
			}
			blk.SetID(deviceID)
			blk.Readonly = drive.Readonly

			vio = blk.Device()

		case "virtio-net-device":
			netdev := cfg.Netdev(dev.Netdev)
			if netdev == nil {
				return fmt.Errorf("unknown netdev: %v", dev.Netdev)
			}
			net, err := virtio.NewNet(hart, virtioRAM, plic, virtioIRQ, mem,
				netdev.IP, netdev.GW, netdev.Hostname, netdev.Domainname)
			if err != nil {
				return err
			}
			mmio.Segments = append(mmio.Segments, net)
			vio = net.Device()

		case "virtio-gpu-device":
			if cfg.NoGraphic {
				continue
			}
			gpudev := cfg.GPUDev(dev.GPU)
			if gpudev == nil {
				return fmt.Errorf("unknown GPU: %v", dev.GPU)
			}
			pointer, err := gpudev.PointerType()
			if err != nil {
				return err
			}
			gpu, err = virtio.NewGPU(hart, virtioRAM, plic, virtioIRQ, mem,
				gpudev.Title, gpudev.Width, gpudev.Height, pointer)
			if err != nil {
				return err
			}
			shmBase = gpu.AssignSHM(shmBase)
			for id, shm := range gpu.SHM {
				fmt.Printf("GPU.SHM[%v]: [%x-%x[\n", id, shm.Start, shm.End)
				mmio.Segments = append(mmio.Segments, shm)
			}

			mmio.Segments = append(mmio.Segments, gpu)
			vio = gpu.Device()

		default:
			return fmt.Errorf("invalid device type: %v", dev.Type)
		}

		virtioRAM = vio.End
		virtioIRQ++
		virtioDevices = append(virtioDevices, vio)
	}

	// If gpu is configured, add its input devices.
	if gpu != nil {
		input := virtio.NewInput(hart, virtioRAM, plic, virtioIRQ, mem,
			gpu.Width, gpu.Height, virtio.InputKeyboard)
		mmio.Segments = append(mmio.Segments, input)
		gpu.KeyboardListener = input

		vio := input.Device()
		virtioRAM = vio.End
		virtioIRQ++
		virtioDevices = append(virtioDevices, vio)

		input = virtio.NewInput(hart, virtioRAM, plic, virtioIRQ, mem,
			gpu.Width, gpu.Height, gpu.Pointer)
		mmio.Segments = append(mmio.Segments, input)
		gpu.MouseListener = input

		vio = input.Device()
		virtioRAM = vio.End
		virtioIRQ++
		virtioDevices = append(virtioDevices, vio)
	}

	// Rest argument files as drives.
	for idx, arg := range args {
		fs, err := os.OpenFile(arg, os.O_RDWR, 06444)
		if err != nil {
			return fmt.Errorf("failed to open disk file %v: %v", arg, err)
		}
		blk := virtio.NewBlk(hart, virtioRAM, plic, virtioIRQ, mem, fs)
		mmio.Segments = append(mmio.Segments, blk)
		blk.SetID(fmt.Sprintf("arg-disk-%d", idx))

		vio := blk.Device()
		virtioRAM = vio.End
		virtioIRQ++
		virtioDevices = append(virtioDevices, vio)
	}

	hart.MMU.MMIO = mmio

	hart.SetMode(isa.ModeM)
	if len(cfg.Symbols) > 0 {
		sm, err := cpu.LoadSystemMap(cfg.Symbols)
		if err != nil {
			return err
		}
		hart.Symtab = sm
	}

	var data []byte
	var entrypoint, entry uint64
	var htifDev *dev.HTIF

	if len(cfg.BIOS) > 0 {
		data, err = os.ReadFile(cfg.BIOS)
		if err != nil {
			return fmt.Errorf("failed to read BIOS: %w", err)
		}
		copy(mem.RAM[mem.Offset(OfsBIOS):], data)
		entrypoint = OfsBIOS
	}

	if len(cfg.Kernel) > 0 {
		htifDev, entry, err = loadKernel(hart, mem, cfg.Kernel)
		if err != nil {
			return fmt.Errorf("failed to read kernel: %w", err)
		}
		if len(cfg.BIOS) == 0 {
			entrypoint = entry
		}
		if htif {
			if htifDev == nil {
				return fmt.Errorf("no HTIF device")
			}
			hart.MMU.Overlay = htifDev
		}
	}

	var initrdSize uint64
	if len(cfg.Initrd) > 0 {
		data, err = os.ReadFile(cfg.Initrd)
		if err != nil {
			return fmt.Errorf("failed to read initrd: %w", err)
		}
		initrdSize = uint64(len(data))
		copy(mem.RAM[mem.Offset(OfsInitrd):], data)
	}

	dtb := makeDTB(initrdSize, mem, plic, uarts, virtioDevices, cfg)
	copy(mem.RAM[mem.Offset(OfsDTB):], dtb)

	hart.X[isa.A0] = 0
	hart.X[isa.A1] = OfsDTB
	hart.PC = entrypoint

	// Start UARTS.
	for _, u := range uarts {
		go u.Peer.Run(u)
	}

	if gpu != nil {
		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			hart.Run()
			wg.Done()
		}()

		gpu.EventLoop()
		wg.Wait()
	} else {
		err = hart.Run()
		if err != nil {
			return err
		}
	}
	fmt.Printf("CPU: instret: %v, runtime: %v, MIPS: %.2f\n",
		hart.Instret, hart.Runtime,
		float64(hart.Instret/1000000.0)/float64(hart.Runtime/time.Second))
	for _, vio := range virtioDevices {
		vio.Stats()
	}
	if htif && htifDev.ExitStatus != 1 {
		fmt.Printf("HTIF: assertion %v\n", htifDev.ExitStatus>>1)
	}
	return nil
}

func loadKernel(hart *cpu.CPU, mem *memory.Memory, file string) (
	*dev.HTIF, uint64, error) {

	data, err := os.ReadFile(file)
	if err != nil {
		return nil, 0, err
	}

	// Is it ELF?
	if len(data) < 4 || !bytes.Equal(data[:4], []byte{0x7f, 0x45, 0x4c, 0x46}) {
		// Raw image.
		copy(mem.RAM[mem.Offset(OfsKernel):], data)
		return nil, OfsKernel, nil
	}

	// ELF kernel.
	f, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	fmt.Printf("ELF: entry=%x\n", f.Entry)

	for _, prog := range f.Progs {
		switch prog.Type {
		case elf.PT_LOAD:
			fmt.Printf("ELF: loading %v bytes to %x\n", prog.Memsz, prog.Paddr)
			if !mem.Contains(prog.Paddr) ||
				!mem.Contains(prog.Paddr+prog.Memsz-1) {
				return nil, 0, fmt.Errorf("prog out of range: %x...%x",
					prog.Paddr, prog.Paddr+prog.Memsz)
			}
			n, err := prog.ReadAt(mem.RAM[mem.Offset(prog.Paddr):], 0)
			if n == 0 && err != nil {
				return nil, 0, err
			}
		}
	}

	symbols, err := f.Symbols()
	if err != nil {
		return nil, 0, err
	}
	var toAddr, toSize, fromAddr, fromSize uint64
	for _, sym := range symbols {
		switch sym.Name {
		case "tohost":
			toAddr = sym.Value
			toSize = sym.Size
		case "fromhost":
			fromAddr = sym.Value
			fromSize = sym.Size
		}
	}
	if false {
		fmt.Printf("Entry: %x\n", f.Entry)
		fmt.Printf("Symbols:\n")
		fmt.Printf(" - tohost  : %x/%x\n", toAddr, toSize)
		fmt.Printf(" - fromhost: %x/%x\n", fromAddr, fromSize)
	}

	if toAddr == 0 {
		return nil, f.Entry, nil
	}

	var start uint64
	var size uint64

	if toAddr < fromAddr {
		start = toAddr
		size = fromAddr + fromSize - toAddr
	} else {
		start = fromAddr
		size = toAddr + toSize - fromAddr
	}

	htif := dev.NewHTIF(hart, start, size, toAddr, fromAddr, mem)

	return htif, f.Entry, nil
}

func makeDTB(initrdSize uint64, mem *memory.Memory, plic *dev.PLIC,
	uarts []*dev.UART, virtioDevices []*virtio.MMIO, cfg *SystemConfig) []byte {

	// Initialize FDT buffer
	buf := make([]byte, 65536)
	fdt := gofdt.NewFDT(buf)

	// ---------------------------------------------------------------------
	// Root node
	// ---------------------------------------------------------------------

	fdt.BeginNode("")

	fdt.PropStr("model", "goemu,riscv-emulator")
	fdt.PropStr("compatible", "riscv,virtio")

	// 64-bit addresses/sizes
	fdt.PropU32("#address-cells", 2)
	fdt.PropU32("#size-cells", 2)

	var tab [8]uint32

	// ---------------------------------------------------------------------
	// CPUs
	// ---------------------------------------------------------------------

	fdt.BeginNode("cpus")

	fdt.PropU32("#address-cells", 2)
	fdt.PropU32("#size-cells", 0)
	fdt.PropU32("timebase-frequency", 100000000)

	// -----------------------------------------------------------------
	// CPU0
	// -----------------------------------------------------------------

	fdt.BeginNode("cpu@0")

	fdt.PropStr("device_type", "cpu")
	fdt.PropStr("status", "okay")

	regData := [4]uint32{
		0x00000000,
		0x00000000,
	}
	fdt.PropTabU32("reg", &regData[0], 2)

	// The standard compatible string for the CPU node
	fdt.PropStr("compatible", "riscv")

	fdt.PropU32("clock-frequency", 100000000)

	// The legacy ISA string (Mandatory for many versions)
	// Note: Use 'g' as an alias for 'imafd' to stay compatible
	fdt.PropStr("riscv,isa", "rv64gcv_sstc")

	// Modern granular ISA description
	fdt.PropStr("riscv,isa-base", "rv64i")

	// Critical: These must be passed as individual arguments to the
	// PropStr function so they are encoded as a string list in the
	// blob.
	fdt.PropTabStr("riscv,isa-extensions",
		"i", "m", "a", "f", "d", "c", "v",
		"zicsr", "zifencei", "zicntr", "zihpm",
		"sstc",
	)

	fdt.PropStr("mmu-type", "riscv,sv39")

	// -------------------------------------------------------------
	// CPU local interrupt controller
	// -------------------------------------------------------------

	fdt.BeginNode("interrupt-controller")

	fdt.PropU32("#interrupt-cells", 1)
	fdt.Prop("interrupt-controller", nil, 0)

	fdt.PropStr("compatible", "riscv,cpu-intc")

	// phandle used by CLINT and PLIC
	fdt.PropU32("phandle", 1)

	fdt.EndNode() // interrupt-controller

	fdt.EndNode() // cpu@0

	fdt.EndNode() // cpus

	// ---------------------------------------------------------------------
	// RAM
	// ---------------------------------------------------------------------

	fdt.BeginNodeNum("memory", mem.RAMBase)

	fdt.PropStr("device_type", "memory")
	ramSize := mem.RAMEnd - mem.RAMBase

	tab = [8]uint32{
		uint32(memory.RAMBase >> 32),
		uint32(memory.RAMBase),

		uint32(ramSize >> 32),
		uint32(ramSize),
	}

	fdt.PropTabU32("reg", &tab[0], 4)

	fdt.EndNode() // memory

	// ---------------------------------------------------------------------
	// SoC Peripherals Bus (Crucial wrapper for OpenSBI probing!)
	// ---------------------------------------------------------------------
	fdt.BeginNode("soc")
	fdt.PropStr("compatible", "simple-bus")
	fdt.PropU32("#address-cells", 2)
	fdt.PropU32("#size-cells", 2)
	fdt.Prop("ranges", nil, 0) // Allows pass-through address mapping to root

	// ---------------------------------------------------------------------
	// CLINT
	// ---------------------------------------------------------------------

	fdt.BeginNodeNum("clint", CLINTBase)

	fdt.PropStr("compatible", "riscv,clint0")

	tab = [8]uint32{
		uint32(CLINTBase >> 32),
		uint32(CLINTBase),

		uint32(CLINTSize >> 32),
		uint32(CLINTSize),
	}

	fdt.PropTabU32("reg", &tab[0], 4)

	// interrupts-extended:
	//   <phandle interrupt-id>
	//
	// 3 = machine software interrupt
	// 7 = machine timer interrupt
	//
	tab = [8]uint32{
		1, 3,
		1, 7,
	}

	fdt.PropTabU32("interrupts-extended", &tab[0], 4)

	fdt.EndNode() // clint

	// ---------------------------------------------------------------------
	// PLIC
	// ---------------------------------------------------------------------

	fdt.BeginNodeNum("plic", plic.Start)

	fdt.PropStr("compatible", "sifive,plic-1.0.0")

	plicSize := plic.End - plic.Start
	tab = [8]uint32{
		uint32(plic.Start >> 32),
		uint32(plic.Start),

		uint32(plicSize >> 32),
		uint32(plicSize),
	}

	fdt.PropTabU32("reg", &tab[0], 4)

	fdt.PropU32("#interrupt-cells", 1)
	fdt.Prop("interrupt-controller", nil, 0)

	// Number of interrupt sources supported. This is defined in PLIC.
	fdt.PropU32("riscv,ndev", plic.MaxInterrupts)

	// PLIC phandle
	fdt.PropU32("phandle", 2)

	// interrupts-extended:
	//   		 1 = cpu0's phandle
	//  		11 = machine external interrupt
	//   		 9 = supervisor external interrupt
	//	0xffffffff = not connected
	//
	tab = [8]uint32{
		1, 11,
		1, 9,
	}

	fdt.PropTabU32("interrupts-extended", &tab[0], 4)

	fdt.EndNode() // plic

	// ---------------------------------------------------------------------
	// UART (16550A)
	// ---------------------------------------------------------------------

	for _, uart := range uarts {
		fdt.BeginNodeNum("uart", uart.Start)

		fdt.PropStr("compatible", "ns16550a")

		size := uart.End - uart.Start
		tab = [8]uint32{
			uint32(uart.Start >> 32),
			uint32(uart.Start),

			uint32(size >> 32),
			uint32(size),
		}

		fdt.PropTabU32("reg", &tab[0], 4)

		tab = [8]uint32{24000000}
		fdt.PropTabU32("clock-frequency", &tab[0], 1)

		fdt.PropU32("reg-shift", 0)
		fdt.PropU32("reg-io-width", 1)

		// UART interrupt comes from PLIC
		tab = [8]uint32{2, uart.IRQ} // phandle=2 (PLIC), irq source=UARTIRQ
		fdt.PropTabU32("interrupts-extended", &tab[0], 2)

		fdt.EndNode()
	}

	// Generic Syscon Register Block.
	fdt.BeginNodeNum("syscon", SysconBase)

	// "syscon" and "simple-mfd" force Linux to initialize it as a
	// multi-function register array
	fdt.PropTabStr("compatible", "syscon", "simple-mfd")
	regData = [4]uint32{
		uint32(SysconBase >> 32), uint32(SysconBase),
		uint32(SysconSize >> 32), uint32(SysconSize),
	}
	fdt.PropTabU32("reg", &regData[0], 4)
	fdt.PropU32("phandle", 3) // Assign a unique phandle to reference this block

	// Syscon Poweroff Controller.
	fdt.BeginNode("poweroff")
	fdt.PropStr("compatible", "syscon-poweroff")
	fdt.PropU32("regmap", 3) // References phandle 3 (our syscon node above)
	fdt.PropU32("offset", 0x0)
	fdt.PropU32("value", dev.PoweroffMagic)
	fdt.PropU32("priority", 100)
	fdt.EndNode()

	// Syscon Reboot Controller.
	fdt.BeginNode("reboot")
	fdt.PropStr("compatible", "syscon-reboot")
	fdt.PropU32("regmap", 3) // References phandle 3 (our syscon node above)
	fdt.PropU32("offset", 0x4)
	fdt.PropU32("value", dev.RebootMagic)
	fdt.PropU32("priority", 200)
	fdt.EndNode()

	fdt.EndNode() // Syscon

	// ---------------------------------------------------------------------
	// Google Goldfish RTC (Real-Time Clock)
	// ---------------------------------------------------------------------
	fdt.BeginNodeNum("rtc", RTCBase)
	fdt.PropStr("compatible", "google,goldfish-rtc")

	// Map to physical address 0x10100000 with a size of 0x1000 (4KB page)
	regData = [4]uint32{
		uint32(RTCBase >> 32), uint32(RTCBase),
		uint32(RTCSize >> 32), uint32(RTCSize),
	}
	fdt.PropTabU32("reg", &regData[0], 4)

	// Connect to PLIC (phandle=2).
	rtcInterrupts := [2]uint32{2, RTCIRQ}
	fdt.PropTabU32("interrupts-extended", &rtcInterrupts[0], 2)
	fdt.EndNode()

	// Create VirtualIO devices.
	for _, dev := range virtioDevices {
		fdt.BeginNodeNum(dev.Name, dev.Start)
		fdt.PropStr("compatible", "virtio,mmio")

		// Address and size.
		size := dev.End - dev.Start
		regData = [4]uint32{
			uint32(dev.Start >> 32), uint32(dev.Start),
			uint32(size >> 32), uint32(size),
		}
		fdt.PropTabU32("reg", &regData[0], 4)

		// Connect to PLIC (phandle=2).
		interrupts := [2]uint32{2, dev.IRQ}
		fdt.PropTabU32("interrupts-extended", &interrupts[0], 2)
		fdt.EndNode()
	}

	fdt.EndNode() // Close the "soc" node wrapper

	// ---------------------------------------------------------------------
	// chosen
	// ---------------------------------------------------------------------

	fdt.BeginNode("chosen")

	fdt.PropStr("bootargs", cfg.Append)

	if initrdSize > 0 {
		// Linux expects these properties to define the physical
		// address boundaries of the ramdisk
		tab = [8]uint32{
			uint32(OfsInitrd >> 32),
			uint32(OfsInitrd),
		}
		fdt.PropTabU32("linux,initrd-start", &tab[0], 2)

		initrdEnd := OfsInitrd + initrdSize
		tab = [8]uint32{
			uint32(initrdEnd >> 32),
			uint32(initrdEnd),
		}
		fdt.PropTabU32("linux,initrd-end", &tab[0], 2)
	}

	fdt.PropStr("stdout-path", "/soc/uart@10000000:115200n8")

	fdt.EndNode() // chosen

	// ---------------------------------------------------------------------
	// End root node
	// ---------------------------------------------------------------------

	fdt.EndNode()

	// Generate final DTB
	size := fdt.Output()

	dtb := buf[:size]

	if len(cfg.DumpDTB) > 0 {
		os.WriteFile(cfg.DumpDTB, dtb, 0644)
	}

	return dtb
}
