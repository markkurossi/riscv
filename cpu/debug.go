//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package cpu

import (
	"fmt"

	"github.com/markkurossi/riscv/isa"
)

func (cpu *CPU) kernelMap(vaddr uint64) (uint64, *SymEntry) {
	// XXX resolve the offsets from cpu.Mem.

	// OpenSBI range: no kernel symbols here
	if vaddr >= 0x80000000 && vaddr < 0x80200000 {
		return vaddr, nil
	}
	if vaddr >= 0x80200000 && vaddr < 0x100000000 {
		// Physical address during early boot (MMU off).  Kernel is
		// loaded at:
		//
		//   0x80200000 physical = 0xffffffff80000000 virtual
		//
		// delta:
		//
		//  0xffffffff80000000 - 0x80200000 = 0xffffffff7fe00000
		vaddr = vaddr - 0x200000 + 0xffffffff00000000
	}
	if cpu.Symtab == nil {
		return vaddr, nil
	}
	return vaddr, cpu.Symtab.Resolve(vaddr)
}

func (cpu *CPU) kernelToPhysical(vaddr uint64) uint64 {
	// OpenSBI range: no kernel symbols here
	if vaddr >= 0x80000000 && vaddr < 0x80200000 {
		return vaddr
	}
	if vaddr >= 0x80200000 && vaddr < 0x100000000 {
		return vaddr
	}
	// Virtual address with MMU on.  Kernel is
	// loaded at:
	//
	//   0x80200000 physical = 0xffffffff80000000 virtual
	//
	// delta:
	//
	//  0xffffffff80000000 - 0x80200000 = 0xffffffff7fe00000
	return vaddr - 0xffffffff00000000 + 0x200000
}

func (cpu *CPU) disassembleKernel(vaddr uint64) {
	_, entry := cpu.kernelMap(vaddr)
	if entry == nil {
		return
	}
	mem := cpu.MMU.Mem

	start := cpu.kernelToPhysical(entry.Start)
	if start < mem.RAMBase || start-mem.RAMBase > uint64(len(mem.RAM)) {
		return
	}

	end := cpu.kernelToPhysical(entry.End)
	if end < mem.RAMBase || end-mem.RAMBase > uint64(len(mem.RAM)) {
		return
	}

	fmt.Printf("Disassembly of exception function:\n")

	fmt.Printf("%v <%s>:\n", fmtAddr(entry.Start), entry.Name)

	var size uint64
	for i := start; i < end; i += size {
		var raw uint32
		var instr isa.Instr
		var err error

		if mem.RAM[mem.Offset(i)]&0b11 == 0b11 {
			raw = bo.Uint32(mem.RAM[mem.Offset(i):])
			instr, err = isa.Decode(raw)
			size = 4
		} else {
			raw = uint32(bo.Uint16(mem.RAM[mem.Offset(i):]))
			instr, err = isa.DecodeC(uint16(raw))
			size = 2
		}
		if err != nil {
			fmt.Printf("disassembleKernel: %v\n", err)
			return
		}

		pc := entry.Start + i - start

		addr := fmtAddr(pc)
		if size == 4 {
			fmt.Printf("%s:  %08x   %v", addr, raw, instr)
		} else {
			fmt.Printf("%s:  %04x       %v", addr, raw, instr)
		}
		if vaddr == pc {
			fmt.Printf("\t# offending instruction")
		}
		fmt.Println()
	}
	fmt.Printf("End of exception function disassembly\n")
}
