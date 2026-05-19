//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package main

import (
	"fmt"
	"os"

	"github.com/markkurossi/gofdt"
	"github.com/markkurossi/riscv/cpu"
	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/kernel"
	"github.com/markkurossi/riscv/memory"
	"github.com/markkurossi/riscv/mmu"
)

const (
	OfsBIOS   = 0x8000_0000
	OfsKernel = 0x8020_0000
	OfsDTB    = 0x8220_0000
	OfsInitrd = 0x8800_0000

	UARTBase = 0x10000000
	UARTSize = 256

	CLINTBase = 0x2000000
	CLINTSize = 0x10000

	PLICBase = 0x0c000000
	PLICSize = 0x04000000
)

func systemEmulation(params kernel.Params,
	bios, kernel, initrd, symbols string) error {

	core := &cpu.CPU{
		Trace: params.CPUtrace,
	}

	mem := memory.New(memory.RAMBase, 0x20000000)
	rom := &ROM{
		Hart: core,
		Segments: []mmu.ROM{
			&UART{
				Hart:  core,
				Start: UARTBase,
				End:   UARTBase + UARTSize,
				Color: params.Color,
			},
			&CLINT{
				Hart:  core,
				Start: CLINTBase,
				End:   CLINTBase + CLINTSize,
			},
			&PLIC{
				Hart:  core,
				Start: PLICBase,
				End:   PLICBase + PLICSize,
			},
		},
	}

	core.MMU = &mmu.MMU{
		Hart: core,
		Mem:  mem,
		ROM:  rom,
	}

	core.SetMode(isa.ModeM)
	if len(symbols) > 0 {
		sm, err := cpu.LoadSystemMap(symbols)
		if err != nil {
			return err
		}
		core.Symtab = sm
	}

	data, err := os.ReadFile(bios)
	if err != nil {
		return fmt.Errorf("failed to read bios: %w", err)
	}
	copy(mem.RAM[mem.Offset(OfsBIOS):], data)

	data, err = os.ReadFile(kernel)
	if err != nil {
		return fmt.Errorf("failed to read kernel: %w", err)
	}
	copy(mem.RAM[mem.Offset(OfsKernel):], data)

	var initrdSize uint64
	if len(initrd) > 0 {
		data, err = os.ReadFile(initrd)
		if err != nil {
			return fmt.Errorf("failed to read initrd: %w", err)
		}
		initrdSize = uint64(len(data))
		copy(mem.RAM[mem.Offset(OfsInitrd):], data)
	}

	dtb := makeDTB(initrdSize)
	copy(mem.RAM[mem.Offset(OfsDTB):], dtb)

	core.X[isa.A0] = 0
	core.X[isa.A1] = OfsDTB
	core.PC = OfsBIOS

	return core.Run()
}

var (
	_ mmu.ROM = &ROM{}
	_ mmu.ROM = &UART{}
	_ mmu.ROM = &CLINT{}
)

type ROM struct {
	Hart     isa.Hart
	Segments []mmu.ROM
}

func (rom *ROM) Contains(paddr uint64) bool {
	for _, seg := range rom.Segments {
		if seg.Contains(paddr) {
			return true
		}
	}
	return false
}

func (rom *ROM) Load8(paddr uint64) (uint8, error) {
	for _, seg := range rom.Segments {
		if seg.Contains(paddr) {
			return seg.Load8(paddr)
		}
	}
	return 0, rom.Hart.Trap(isa.CauseLoadPageFault, paddr, nil)
}

func (rom *ROM) Load16(paddr uint64) (uint16, error) {
	for _, seg := range rom.Segments {
		if seg.Contains(paddr) {
			return seg.Load16(paddr)
		}
	}
	return 0, rom.Hart.Trap(isa.CauseLoadPageFault, paddr, nil)
}

func (rom *ROM) Load32(paddr uint64) (uint32, error) {
	for _, seg := range rom.Segments {
		if seg.Contains(paddr) {
			return seg.Load32(paddr)
		}
	}
	return 0, rom.Hart.Trap(isa.CauseLoadPageFault, paddr, nil)
}

func (rom *ROM) Load64(paddr uint64) (uint64, error) {
	for _, seg := range rom.Segments {
		if seg.Contains(paddr) {
			return seg.Load64(paddr)
		}
	}
	return 0, rom.Hart.Trap(isa.CauseLoadPageFault, paddr, nil)
}

func (rom *ROM) Store8(paddr, v uint64) error {
	for _, seg := range rom.Segments {
		if seg.Contains(paddr) {
			return seg.Store8(paddr, v)
		}
	}
	return rom.Hart.Trap(isa.CauseStorePageFault, paddr, nil)
}

func (rom *ROM) Store16(paddr, v uint64) error {
	for _, seg := range rom.Segments {
		if seg.Contains(paddr) {
			return seg.Store16(paddr, v)
		}
	}
	return rom.Hart.Trap(isa.CauseStorePageFault, paddr, nil)
}

func (rom *ROM) Store32(paddr, v uint64) error {
	for _, seg := range rom.Segments {
		if seg.Contains(paddr) {
			return seg.Store32(paddr, v)
		}
	}
	return rom.Hart.Trap(isa.CauseStorePageFault, paddr, nil)
}

func (rom *ROM) Store64(paddr, v uint64) error {
	for _, seg := range rom.Segments {
		if seg.Contains(paddr) {
			return seg.Store64(paddr, v)
		}
	}
	return rom.Hart.Trap(isa.CauseStorePageFault, paddr, nil)
}

func makeDTB(initrdSize uint64) []byte {
	// Initialize FDT buffer
	buf := make([]byte, 65536)
	fdt := gofdt.NewFDT(buf)

	// ---------------------------------------------------------------------
	// Root node
	// ---------------------------------------------------------------------

	fdt.BeginNode("")

	fdt.PropStr("model", "goemu,riscv-emulator")
	fdt.PropStr("compatible", "riscv-virtio")

	// 64-bit addresses/sizes
	fdt.PropU32("#address-cells", 2)
	fdt.PropU32("#size-cells", 2)

	var tab [8]uint32

	// ---------------------------------------------------------------------
	// CPUs
	// ---------------------------------------------------------------------

	fdt.BeginNode("cpus")

	fdt.PropU32("#address-cells", 1)
	fdt.PropU32("#size-cells", 0)
	fdt.PropU32("timebase-frequency", 10000000)

	// -----------------------------------------------------------------
	// CPU0
	// -----------------------------------------------------------------

	fdt.BeginNode("cpu@0")

	fdt.PropStr("device_type", "cpu")
	fdt.PropStr("status", "okay")

	fdt.PropU32("reg", 0)

	// The standard compatible string for the CPU node
	fdt.PropStr("compatible", "riscv")

	// The legacy ISA string (Mandatory for many versions)
	// Note: Use 'g' as an alias for 'imafd' to stay compatible
	fdt.PropStr("riscv,isa", "rv64gc")

	// Modern granular ISA description
	fdt.PropStr("riscv,isa-base", "rv64i")

	// Critical: These must be passed as individual arguments to the PropStr function
	// so they are encoded as a string list in the blob.
	fdt.PropTabStr("riscv,isa-extensions",
		"i", "m", "a", "f", "d", "c", "zicsr", "zifencei", "zicntr", "zihpm",
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

	fdt.BeginNodeNum("memory", memory.RAMBase)

	fdt.PropStr("device_type", "memory")

	tab = [8]uint32{
		uint32(memory.RAMBase >> 32),
		uint32(memory.RAMBase),

		0x0,
		0x20000000, // 512 MB
	}

	fdt.PropTabU32("reg", &tab[0], 4)

	fdt.EndNode() // memory

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

	// Number of interrupt sources supported
	fdt.PropU32("riscv,ndev", 32)

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
	tab = [8]uint32{2, 10} // phandle=2 (PLIC), irq source=10
	fdt.PropTabU32("interrupts-extended", &tab[0], 2)

	fdt.EndNode() // uart

	// ---------------------------------------------------------------------
	// chosen
	// ---------------------------------------------------------------------

	fdt.BeginNode("chosen")

	fdt.PropStr(
		"bootargs",
		// "console=ttyS0,115200 earlycon=uart8250,mmio,0x10000000,115200 keep_bootcon lpj=1000000",
		// "earlycon=sbi console=ttyS0,115200 lpj=1000000",
		// "earlycon=sbi console=ttyS0,115200",
		"earlycon=sbi console=ttyS0,115200 init=/init",
		//"earlycon=uart8250,mmio,0x10000000 console=ttyS0,115200 root=/dev/ram0 rw init=/init norandmaps",
	)

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

	fdt.PropStr("stdout-path", "/uart@10000000:115200n8")

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
