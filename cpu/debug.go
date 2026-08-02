//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package cpu

import (
	"fmt"
	"log"

	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/memory"
)

func (cpu *CPU) disassembleFunction(name string) {
	entry := cpu.Symtab.Lookup(name)
	if entry == nil {
		return
	}

	cpu.disassembleKernel(entry.Start)
}

func (cpu *CPU) kernelToPhys(vaddr uint64) (uint64, bool) {
	if vaddr >= 0x80200000 && vaddr < 0x100000000 {
		// Physical address during early boot (MMU off).
		return vaddr, true
	} else if vaddr >= 0xffffffff80000000 {
		return vaddr - 0xffffffff80000000 + 0x80200000, true
	}
	return 0, false
}

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
		log.Printf("vaddr %x not in kernel map\r\n", vaddr)
		return
	}
	mem := cpu.MMU.Mem

	start := cpu.kernelToPhysical(entry.Start)
	if start < mem.RAMBase || start-mem.RAMBase > uint64(len(mem.RAM)) {
		log.Printf("entry start %x outside of RAM range\r\n", start)
		return
	}

	end := cpu.kernelToPhysical(entry.End)
	if end < mem.RAMBase || end-mem.RAMBase > uint64(len(mem.RAM)) {
		log.Printf("entry end %x outside of RAM range\r\n", end)
		return
	}

	log.Printf("Disassembly of exception function:\r\n")

	log.Printf("%v <%s>:\r\n", fmtAddr(entry.Start), entry.Name)

	var size uint64
	for i := start; i < end; i += size {
		var raw uint32
		var instr isa.Instr
		var err error

		if mem.RAM[mem.Offset(i)]&0b11 == 0b11 {
			raw = memory.Uint32(mem.RAM, mem.Offset(i))
			instr, err = isa.Decode(raw)
			size = 4
		} else {
			raw = uint32(memory.Uint16(mem.RAM, mem.Offset(i)))
			instr, err = isa.DecodeC(uint16(raw))
			size = 2
		}
		if err != nil {
			log.Printf("disassembleKernel: %v\r\n", err)
			return
		}

		pc := entry.Start + i - start

		addr := fmtAddr(pc)
		var line string
		if size == 4 {
			line = fmt.Sprintf("%s:  %08x   %v", addr, raw, instr)
		} else {
			line = fmt.Sprintf("%s:  %04x       %v", addr, raw, instr)
		}
		if vaddr == pc {
			line += "\t# offending instruction"
		}
		log.Printf("%s\r\n", line)
	}
	log.Printf("End of exception function disassembly\r\n")
}
