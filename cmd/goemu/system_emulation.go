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

	UARTBase = 0x10000000
	UARTSize = 256
	UARTIRQ  = 10

	SysconBase = 0x10000100
	SysconSize = 256

	RTCBase = 0x10100000
	RTCSize = 0x1000
	RTCIRQ  = 11

	VirtioROMBase = 0x10008000
	VirtioIRQBase = 1

	CLINTBase = 0x2000000
	CLINTSize = 0x10000

	PLICBase = 0x0c000000
	PLICSize = 0x400000
)

func systemEmulation(params kernel.Params, cfg *SystemConfig) error {

	var ramSize uint64 = 0x20000000
	mem := memory.New(memory.RAMBase, ramSize)

	core := cpu.New(mem)
	core.Trace = params.CPUtrace

	plic := &dev.PLIC{
		Hart:  core,
		Start: PLICBase,
		End:   PLICBase + PLICSize,
	}

	uart := dev.NewUART(core, UARTBase, UARTSize, plic, UARTIRQ,
		params.Color, params.Cooked)

	mmio := &MMIO{
		Hart: core,
		Segments: []mmu.MMIO{
			plic,
			uart,
			dev.NewSyscon(core, SysconBase, SysconSize),
			&dev.GoldfishRTC{
				Hart:  core,
				Start: RTCBase,
				End:   RTCBase + RTCSize,
			},
			&dev.CLINT{
				Hart:  core,
				Start: CLINTBase,
				End:   CLINTBase + CLINTSize,
			},
		},
	}

	// Create VirtIO devices.
	var virtioDevices []*virtio.MMIO

	var virtioROM uint64 = VirtioROMBase
	var virtioIRQ uint32 = VirtioIRQBase

	// Entropy Source / RNG.
	rng := virtio.NewRng(core, virtioROM, plic, virtioIRQ, mem)
	mmio.Segments = append(mmio.Segments, rng)
	virtioROM = rng.End
	virtioIRQ++
	virtioDevices = append(virtioDevices, rng.Device())

	// Devices from the configuration.
	for idx, dev := range cfg.Devices {
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

			dev.blk = virtio.NewBlk(core, virtioROM, plic, virtioIRQ, mem, fs)
			mmio.Segments = append(mmio.Segments, dev.blk)

			deviceID := dev.ID
			if len(deviceID) == 0 {
				deviceID = fmt.Sprintf("goemu-disk-%d", idx)
			}
			dev.blk.SetID(deviceID)
			dev.blk.Readonly = drive.Readonly

			virtioROM = dev.blk.End
			virtioIRQ++

		default:
			return fmt.Errorf("invalid device type: %v", dev.Type)
		}
	}

	core.MMU.MMIO = mmio

	core.SetMode(isa.ModeM)
	if len(cfg.Symbols) > 0 {
		sm, err := cpu.LoadSystemMap(cfg.Symbols)
		if err != nil {
			return err
		}
		core.Symtab = sm
	}

	data, err := os.ReadFile(cfg.BIOS)
	if err != nil {
		return fmt.Errorf("failed to read BIOS: %w", err)
	}
	copy(mem.RAM[mem.Offset(OfsBIOS):], data)

	err = loadKernel(cfg.Kernel, mem)
	if err != nil {
		return fmt.Errorf("failed to read kernel: %w", err)
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

	dtb := makeDTB(initrdSize, mem, virtioDevices, cfg)
	copy(mem.RAM[mem.Offset(OfsDTB):], dtb)

	core.X[isa.A0] = 0
	core.X[isa.A1] = OfsDTB
	core.PC = OfsBIOS

	go uart.Run()

	err = core.Run()
	if err != nil {
		return err
	}
	fmt.Printf("CPU: instret: %v, runtime: %v, MIPS: %.2f\n",
		core.Instret, core.Runtime,
		float64(core.Instret/1000000.0)/float64(core.Runtime/time.Second))
	return nil
}

func loadKernel(file string, mem *memory.Memory) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	// Is it ELF?
	if len(data) < 4 || !bytes.Equal(data[:4], []byte{0x7f, 0x45, 0x4c, 0x46}) {
		// Raw image.
		copy(mem.RAM[mem.Offset(OfsKernel):], data)
		return nil
	}

	// ELF kernel.
	f, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Printf("ELF: entry=%x\n", f.Entry)

	for _, prog := range f.Progs {
		switch prog.Type {
		case elf.PT_LOAD:
			fmt.Printf("ELF: loading %v bytes to %x\n", prog.Memsz, prog.Paddr)
			if !mem.Contains(prog.Paddr) ||
				!mem.Contains(prog.Paddr+prog.Memsz-1) {
				return fmt.Errorf("prog out of range: %x...%x",
					prog.Paddr, prog.Paddr+prog.Memsz)
			}
			n, err := prog.ReadAt(mem.RAM[mem.Offset(prog.Paddr):], 0)
			if n == 0 && err != nil {
				return err
			}
		}
	}

	return nil
}

func makeDTB(initrdSize uint64, mem *memory.Memory,
	virtioDevices []*virtio.MMIO, cfg *SystemConfig) []byte {

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
	fdt.PropStr("riscv,isa", "rv64gc_sstc")

	// Modern granular ISA description
	fdt.PropStr("riscv,isa-base", "rv64i")

	// Critical: These must be passed as individual arguments to the
	// PropStr function so they are encoded as a string list in the
	// blob.
	fdt.PropTabStr("riscv,isa-extensions",
		"i", "m", "a", "f", "d", "c", "zicsr", "zifencei", "zicntr", "zihpm",
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

	fdt.BeginNodeNum("plic", PLICBase)

	fdt.PropStr("compatible", "sifive,plic-1.0.0")

	tab = [8]uint32{
		uint32(PLICBase >> 32),
		uint32(PLICBase),

		uint32(PLICSize >> 32),
		uint32(PLICSize),
	}

	fdt.PropTabU32("reg", &tab[0], 4)

	fdt.PropU32("#interrupt-cells", 1)
	fdt.Prop("interrupt-controller", nil, 0)

	// Number of interrupt sources supported. This is defined in PLIC.
	fdt.PropU32("riscv,ndev", dev.MaxInterrupts)

	// PLIC phandle
	fdt.PropU32("phandle", 2)

	// interrupts-extended:
	//
	// 11 = machine external interrupt
	//  9 = supervisor external interrupt
	//
	tab = [8]uint32{
		1, 0xffffffff, // hart 0 M-mode context (use 0xffffffff = not connected)
		1, 9, // hart 0 S-mode supervisor external interrupt
	}

	fdt.PropTabU32("interrupts-extended", &tab[0], 4)

	fdt.EndNode() // plic

	// ---------------------------------------------------------------------
	// UART (16550A)
	// ---------------------------------------------------------------------

	fdt.BeginNodeNum("uart", UARTBase)

	fdt.PropStr("compatible", "ns16550a")

	tab = [8]uint32{
		uint32(UARTBase >> 32),
		uint32(UARTBase),

		uint32(UARTSize >> 32),
		uint32(UARTSize),
	}

	fdt.PropTabU32("reg", &tab[0], 4)

	tab = [8]uint32{24000000}
	fdt.PropTabU32("clock-frequency", &tab[0], 1)

	fdt.PropU32("reg-shift", 0)
	fdt.PropU32("reg-io-width", 1)

	// UART interrupt comes from PLIC
	tab = [8]uint32{2, UARTIRQ} // phandle=2 (PLIC), irq source=UARTIRQ
	fdt.PropTabU32("interrupts-extended", &tab[0], 2)

	fdt.EndNode()

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
	for _, dev := range cfg.Devices {
		switch dev.Type {
		case "virtio-blk-device": // VirtIO Block Device.
			fdt.BeginNodeNum(dev.blk.Name, dev.blk.Start)
			fdt.PropStr("compatible", "virtio,mmio")

			// Address and size.
			size := dev.blk.End - dev.blk.Start
			regData = [4]uint32{
				uint32(dev.blk.Start >> 32), uint32(dev.blk.Start),
				uint32(size >> 32), uint32(size),
			}
			fdt.PropTabU32("reg", &regData[0], 4)

			// Connect to PLIC (phandle=2).
			interrupts := [2]uint32{2, dev.blk.IRQ}
			fdt.PropTabU32("interrupts-extended", &interrupts[0], 2)
			fdt.EndNode()

		default:
			panic("invalid device type")
		}
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

	os.WriteFile("goemu.dtb", dtb, 0644)

	return dtb
}
