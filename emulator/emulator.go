//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

// Package emulator implements the RISC-V emulator.
package emulator

import (
	"crypto/rand"
	"debug/elf"
	"encoding/hex"
	"fmt"

	"github.com/markkurossi/riscv/isa"
)

type Emulator struct {
	CPU *CPU
	Mem *Memory
}

func New() *Emulator {
	mem := new(Memory)

	mem.MmapStart = 0x4000000000
	mem.MmapEnd = mem.MmapStart

	stack := NewStack(0x7ffff000, 1<<20)
	mem.Add(stack)

	cpu := &CPU{
		Mem:     mem,
		Syscall: LinuxSyscall,
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

			if seg.Write && seg.End > emu.Mem.HeapEnd {
				fmt.Printf("seg.End    : 0x%x\n", seg.End)
				emu.Mem.HeapEnd = (seg.End + 4095) & ^uint64(0xfff)
				emu.Mem.HeapStart = emu.Mem.HeapEnd
				fmt.Printf(" => HeapEnd: 0x%x\n", emu.Mem.HeapEnd)
			}
		}
	}

	emu.CPU.PC = f.Entry

	return nil
}

func (emu *Emulator) Run(argv []string, envp []string) error {
	var argvPtrs []uint64
	var envpPtrs []uint64

	fmt.Println("argv:")
	for i, v := range argv {
		fmt.Printf(" - argv[%d]:%v\n", i, v)
		err := emu.PushCString(v)
		if err != nil {
			return err
		}
		argvPtrs = append(argvPtrs, emu.CPU.X[isa.Sp])
	}

	fmt.Println("envp:")
	for i, v := range envp {
		fmt.Printf(" - envp[%d]:%v\n", i, v)
		err := emu.PushCString(v)
		if err != nil {
			return err
		}
		envpPtrs = append(envpPtrs, emu.CPU.X[isa.Sp])
	}

	var random [16]byte
	_, err := rand.Read(random[:])
	if err != nil {
		return err
	}
	err = emu.PushData(random[:])
	if err != nil {
		return err
	}
	atRandom := emu.CPU.X[isa.Sp]

	// Calculate the exact number of 8-byte words we are about to
	// push. We ignore auxiliary vector as it is always multiple of 16
	// bytes.
	//
	//	argc (1) + argv ptrs (len) + NULL (1) + envp ptrs (len) + NULL (1)
	wordsToPush := 1 + len(argvPtrs) + 1 + len(envpPtrs) + 1

	// Align sp to 16-bytes.
	emu.CPU.X[isa.Sp] &^= 0b1111

	// If pushing the words throws us off 16-byte alignment, push a
	// pad word.
	if (wordsToPush*8)%16 != 0 {
		emu.Push(0)
	}

	// Push Auxiliary Vector terminator (AT_NULL = 0, val = 0)

	emu.Push(AtNull)
	emu.Push(0)

	emu.Push(atRandom)
	emu.Push(AtRandom)

	emu.Push(4096)
	emu.Push(AtPagesz)

	// Push environment pointers.

	if err := emu.Push(0); err != nil {
		return err
	}
	for i := len(envpPtrs) - 1; i >= 0; i-- {
		if err := emu.Push(envpPtrs[i]); err != nil {
			return err
		}
	}

	// Push argv and argc.

	if err := emu.Push(0); err != nil {
		return err
	}
	for i := len(argvPtrs) - 1; i >= 0; i-- {
		if err := emu.Push(argvPtrs[i]); err != nil {
			return err
		}
	}
	if err := emu.Push(uint64(len(argvPtrs))); err != nil {
		return err
	}

	seg, ofs, err := emu.Mem.Map(emu.CPU.X[isa.Sp], 1)
	if err != nil {
		return err
	}
	fmt.Printf("Stack:\n%s", hex.Dump(seg.Data[ofs:]))
	if false {
		return fmt.Errorf("debug")
	}

	return emu.CPU.Run()
}

func (emu *Emulator) Push(val uint64) error {
	emu.CPU.X[isa.Sp] -= 8
	return emu.Mem.Store64(emu.CPU.X[isa.Sp], val)
}

func (emu *Emulator) PushCString(val string) error {
	emu.CPU.X[isa.Sp]--

	err := emu.Mem.Store8(emu.CPU.X[isa.Sp], uint64(0))
	if err != nil {
		return err
	}
	bytes := []byte(val)

	emu.CPU.X[isa.Sp] -= uint64(len(bytes))
	return emu.Mem.StoreData(emu.CPU.X[isa.Sp], bytes)
}

func (emu *Emulator) PushData(data []byte) error {
	emu.CPU.X[isa.Sp] -= uint64(len(data))
	return emu.Mem.StoreData(emu.CPU.X[isa.Sp], data)
}
