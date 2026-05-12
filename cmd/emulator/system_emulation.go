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
	OfsDTB    = 0x8400_0000
	OfsInitrd = 0x8800_0000

	UARTBase = 0x10000000
	UARTSize = 256

	CLINTBase = 0x2000000
	CLINTSize = 0x10000
)

func systemEmulation(params kernel.Params, bios, kernel string) error {
	mem := memory.New(0x20000000)
	rom := &ROM{
		Segments: []mmu.ROM{
			&UART{
				Start: UARTBase,
				End:   UARTBase + UARTSize,
			},
			&CLINT{
				Start: CLINTBase,
				End:   CLINTBase + CLINTSize,
			},
		},
	}

	core := &cpu.CPU{
		MMU: &mmu.MMU{
			Satp: mmu.SatpModeBare,
			Mem:  mem,
			ROM:  rom,
		},
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

	dtb := makeDTB()
	copy(mem.RAM[mem.Offset(OfsDTB):], dtb)

	core.X[isa.A0] = 0
	core.X[isa.A1] = OfsDTB
	core.PC = OfsBIOS

	core.TrapHandler = func(core *cpu.CPU, trap *isa.Trap) (bool, error) {
		return handleTrap(core, trap, mem)
	}

	return core.Run()
}

var (
	_ mmu.ROM = &ROM{}
	_ mmu.ROM = &UART{}
	_ mmu.ROM = &CLINT{}
)

type ROM struct {
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
	return 0, isa.NewTrap(0, isa.CauseLoadPageFault, paddr, nil)
}

func (rom *ROM) Load16(paddr uint64) (uint16, error) {
	return 0, nil
}

func (rom *ROM) Load32(paddr uint64) (uint32, error) {
	return 0, nil
}

func (rom *ROM) Load64(paddr uint64) (uint64, error) {
	return 0, nil
}

func (rom *ROM) Store8(paddr, v uint64) error {
	for _, seg := range rom.Segments {
		if seg.Contains(paddr) {
			return seg.Store8(paddr, v)
		}
	}
	return isa.NewTrap(0, isa.CauseStorePageFault, paddr, nil)
}

func (rom *ROM) Store16(paddr, v uint64) error {
	for _, seg := range rom.Segments {
		if seg.Contains(paddr) {
			return seg.Store16(paddr, v)
		}
	}
	return isa.NewTrap(0, isa.CauseStorePageFault, paddr, nil)
}

func (rom *ROM) Store32(paddr, v uint64) error {
	for _, seg := range rom.Segments {
		if seg.Contains(paddr) {
			return seg.Store32(paddr, v)
		}
	}
	return isa.NewTrap(0, isa.CauseStorePageFault, paddr, nil)
}

func (rom *ROM) Store64(paddr, v uint64) error {
	for _, seg := range rom.Segments {
		if seg.Contains(paddr) {
			return seg.Store64(paddr, v)
		}
	}
	return isa.NewTrap(0, isa.CauseStorePageFault, paddr, nil)
}

func handleTrap(core *cpu.CPU, trap *isa.Trap, mem *memory.Memory) (
	bool, error) {

	fmt.Printf("goemu: %v\n", trap)
	switch trap.Cause {
	case isa.CauseBreakpoint:
		core.PC = core.M.Tvec
		return true, nil
	}

	return false, nil
}

func makeDTB() []byte {
	// 1. Initialize the FDT builder with a buffer
	buf := make([]byte, 4096)
	fdt := gofdt.NewFDT(buf)

	// 2. Start Root Node
	fdt.BeginNode("")
	// Basic system properties
	fdt.PropStr("model", "goemu,riscv-emulator")
	fdt.PropStr("compatible", "riscv-virtio")
	fdt.PropU32("#address-cells", 2) // 64-bit addresses (2x32-bit)
	fdt.PropU32("#size-cells", 2)    // 64-bit sizes

	// 3. CPU Topology
	fdt.BeginNode("cpus")
	fdt.PropU32("#address-cells", 1)
	fdt.PropU32("#size-cells", 0)
	fdt.PropU32("timebase-frequency", 10000000) // 10MHz standard

	fdt.BeginNode("cpu@0")
	fdt.PropStr("device_type", "cpu")
	fdt.PropU32("reg", 0)
	fdt.PropStr("status", "okay")
	fdt.PropStr("compatible", "riscv")
	fdt.PropStr("riscv,isa", "rv64imafdc")
	fdt.PropStr("mmu-type", "riscv,sv39")

	// CPU Interrupt Controller (required for S-Mode)
	fdt.BeginNode("interrupt-controller")
	fdt.PropU32("#interrupt-cells", 1)
	fdt.Prop("interrupt-controller", nil, 0)
	fdt.PropStr("compatible", "riscv,cpu-intc")
	fdt.PropU32("phandle", 1) // Reference handle for PLIC
	fdt.EndNode()
	fdt.EndNode()
	fdt.EndNode()

	// 4. System Memory (RAM)
	// Assuming RAM starts at 0x80000000 and is 512MB
	fdt.BeginNodeNum("memory", memory.RAMBase)
	fdt.PropStr("device_type", "memory")
	// reg is [address_high, address_low, size_high, size_low]
	tab := [4]uint32{
		uint32(memory.RAMBase) >> 32, uint32(memory.RAMBase),
		0x0, 0x20000000,
	}
	fdt.PropTabU32("reg", &tab[0], 4)
	fdt.EndNode()

	// 5. CLINT (Timer and IPI)
	// Standard address for QEMU virt is 0x2000000
	fdt.BeginNodeNum("clint", CLINTBase)
	fdt.PropStr("compatible", "riscv,clint0")
	tab = [4]uint32{
		uint32(CLINTBase) >> 32, uint32(CLINTBase),
		uint32(CLINTSize) >> 32, uint32(CLINTSize),
	}
	fdt.PropTabU32("reg", &tab[0], 4)
	// Link it to the CPUs
	tab = [4]uint32{
		1, 3, // Hart 0 M-Software
		1, 7, // Hart 0 M-Timer
	}
	fdt.PropTabU32("interrupts-extended", &tab[0], 4)
	fdt.EndNode()

	// 6. Peripherals (UART 16550A)
	fdt.BeginNodeNum("uart", UARTBase)
	fdt.PropStr("compatible", "ns16550a")
	tab = [4]uint32{
		uint32(UARTBase) >> 32, uint32(UARTBase),
		uint32(UARTSize) >> 32, uint32(UARTSize),
	}
	fdt.PropTabU32("reg", &tab[0], 4)
	fdt.PropU32("clock-frequency", 3686400)
	fdt.PropU32("interrupt-parent", 1) // Link to CPU INTC phandle
	fdt.PropU32("interrupts", 10)
	fdt.EndNode()

	// 7. Chosen node (Boot arguments)
	fdt.BeginNode("chosen")
	// Tells Linux to use the UART for its console
	fdt.PropStr("bootargs", "console=ttyS0 earlycon=uart8250,mmio,0x10000000")
	fdt.PropStr("stdout-path", "/uart@10000000")
	fdt.EndNode()

	fdt.EndNode() // End Root

	// 7. Finish and get binary blob
	size := fdt.Output()

	return buf[:size]
}
