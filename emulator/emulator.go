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
	Verbose bool

	CPU *CPU
	Mem *Memory

	ProgBase    uint64
	ProgBaseEnd uint64

	Prog   *fileInfo
	Interp *fileInfo
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
		CPU:         cpu,
		Mem:         mem,
		ProgBase:    0x400000,
		ProgBaseEnd: 0x400000,
	}
}

func (emu *Emulator) Debugf(msg string, args ...interface{}) {
	if !emu.Verbose {
		return
	}
	fmt.Printf(msg, args...)
}

func (emu *Emulator) LoadELF(file string) error {

	info, err := emu.load(file)
	if err != nil {
		return err
	}

	emu.Prog = info
	if len(emu.Prog.Interp) > 0 {
		// Dynamically linked executable.
		fmt.Printf("PT_INTERP: %v\n", emu.Prog.Interp)

		info, err = emu.load("image" + emu.Prog.Interp)
		if err != nil {
			return err
		}
		emu.Interp = info
	}

	return nil
}

type fileInfo struct {
	Dynamic bool
	Phdr    uint64
	Phnum   uint64
	Base    uint64
	Entry   uint64
	Interp  string
}

func (emu *Emulator) load(file string) (*fileInfo, error) {
	f, err := elf.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fmt.Printf("File:\n")
	fmt.Printf(" - Class     : %v\n", f.Class)
	fmt.Printf(" - Data      : %v\n", f.Data)
	fmt.Printf(" - Version   : %v\n", f.Version)
	fmt.Printf(" - OSABI     : %v\n", f.OSABI)
	fmt.Printf(" - ABIVersion: %v\n", f.ABIVersion)
	fmt.Printf(" - ByteOrder : %v\n", f.ByteOrder)
	fmt.Printf(" - Type      : %v\n", f.Type)
	fmt.Printf(" - Machine   : %v\n", f.Machine)
	fmt.Printf(" - Entry     : %x\n", f.Entry)

	info := &fileInfo{
		Dynamic: f.Type == elf.ET_DYN,
		Phnum:   uint64(len(f.Progs)),
		Entry:   f.Entry,
	}
	if info.Dynamic {
		info.Entry += emu.ProgBase
	}

	for idx, prog := range f.Progs {
		fmt.Printf("Prog %v\n", idx)
		fmt.Printf(" - Type : %v\n", prog.Type)
		fmt.Printf(" - Flags: %v\n", prog.Flags)
		fmt.Printf(" - Vaddr: %x\n", prog.Vaddr)
		fmt.Printf(" - Memsz: %x\n", prog.Memsz)
		fmt.Printf(" - Align: %x\n", prog.Align)

		switch prog.Type {
		case elf.PT_PHDR:
			info.Phdr = prog.Vaddr
			if info.Dynamic {
				info.Phdr += emu.ProgBase
			}

		case elf.PT_LOAD:
			data := make([]byte, prog.Memsz)
			_, err = prog.ReadAt(data[:prog.Filesz], 0)
			if err != nil {
				return nil, err
			}
			vaddr := prog.Vaddr
			if info.Dynamic {
				vaddr += emu.ProgBase

				end := vaddr + prog.Memsz + 4095
				end &= ^uint64(0xfff)

				if end > emu.ProgBaseEnd {
					emu.ProgBaseEnd = end
				}
			}
			if info.Base == 0 {
				info.Base = vaddr
			}

			fmt.Printf(" @ Vaddr: %x\n", vaddr)

			seg := &Segment{
				Start: vaddr,
				End:   vaddr + prog.Memsz,
				Data:  data,
				Read:  prog.Flags&elf.PF_R != 0,
				Write: prog.Flags&elf.PF_W != 0,
				Exec:  prog.Flags&elf.PF_X != 0,
			}
			if seg.Exec && info.Dynamic && false {
				fmt.Printf(" ! Entry: %x + %x =>", info.Entry, seg.Start)
				info.Entry += seg.Start
				fmt.Printf(" %x\n", info.Entry)
			}

			emu.Mem.Add(seg)

			if seg.Write && seg.End > emu.Mem.HeapEnd {
				fmt.Printf("seg.End    : 0x%x\n", seg.End)
				emu.Mem.HeapEnd = (seg.End + 4095) & ^uint64(0xfff)
				emu.Mem.HeapStart = emu.Mem.HeapEnd
				fmt.Printf(" => HeapEnd: 0x%x\n", emu.Mem.HeapEnd)
			}

		case elf.PT_INTERP:
			data := make([]byte, prog.Memsz)
			_, err = prog.ReadAt(data, 0)
			if err != nil {
				return nil, err
			}
			var end int
			for end = len(data) - 1; end > 0 && data[end] == 0; end-- {
			}
			end++
			info.Interp = string(data[:end])
		}
	}

	// Update base address for the next ELF file.
	emu.ProgBase = emu.ProgBaseEnd

	return info, nil
}

func (emu *Emulator) Run(argv []string, envp []string) error {
	var argvPtrs []uint64
	var envpPtrs []uint64

	emu.Debugf("argv:\n")
	for i, v := range argv {
		emu.Debugf(" - argv[%d]:%v\n", i, v)
		err := emu.PushCString(v)
		if err != nil {
			return err
		}
		argvPtrs = append(argvPtrs, emu.CPU.X[isa.Sp])
	}

	emu.Debugf("envp:\n")
	for i, v := range envp {
		emu.Debugf(" - envp[%d]:%v\n", i, v)
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
	if wordsToPush%2 != 0 {
		emu.Push(0)
	}

	// Push Auxiliary Vector terminator (AT_NULL = 0, val = 0)

	emu.Push(AtNull)
	emu.Push(0)

	emu.Push(atRandom)
	emu.Push(AtRandom)

	emu.Push(4096)
	emu.Push(AtPagesz)

	emu.Push(0x112d)
	emu.Push(AtHwcap)

	emu.Push(100)
	emu.Push(AtClktck)

	emu.Push(emu.Prog.Phdr)
	emu.Push(AtPhdr)

	emu.Push(56)
	emu.Push(AtPhent)

	emu.Push(emu.Prog.Phnum)
	emu.Push(AtPhnum)

	if emu.Interp != nil {
		emu.Push(emu.Interp.Base)
		emu.Push(AtBase)
	} else {
		emu.Push(0)
		emu.Push(AtBase)
	}

	emu.Push(emu.Prog.Entry)
	emu.Push(AtEntry)

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
	emu.Debugf("Stack:\n%s", hex.Dump(seg.Data[ofs:]))

	if emu.Interp != nil {
		emu.CPU.PC = emu.Interp.Entry
	} else {
		emu.CPU.PC = emu.Prog.Entry
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
