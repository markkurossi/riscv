//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

// Package emulator implements the RISC-V emulator.
package emulator

import (
	"debug/elf"

	"github.com/markkurossi/riscv/isa"
)

type Emulator struct {
	CPU *CPU
	Mem *Memory
}

func New() *Emulator {
	mem := new(Memory)

	stack := NewStack(0x7ffff000, 1<<20)
	mem.Add(stack)

	cpu := &CPU{
		Mem: mem,
	}
	cpu.X[isa.Sp] = stack.End

	return &Emulator{
		CPU: cpu,
		Mem: mem,
	}
}

func (emu *Emulator) LoadELF(file string) error {
	f, err := elf.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, prog := range f.Progs {
		if prog.Type == elf.PT_LOAD {
			data := make([]byte, prog.Memsz)
			_, err = prog.ReadAt(data[:prog.Filesz], 0)
			if err != nil {
				return err
			}

			seg := &Segment{
				Start: prog.Vaddr,
				End:   prog.Vaddr + prog.Memsz,
				Data:  data,
				Read:  prog.Flags&elf.PF_R != 0,
				Write: prog.Flags&elf.PF_W != 0,
				Exec:  prog.Flags&elf.PF_X != 0,
			}

			emu.Mem.Add(seg)
		}
	}

	emu.CPU.PC = f.Entry

	return nil
}
