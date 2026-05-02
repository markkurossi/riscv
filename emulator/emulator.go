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
	"os"

	"github.com/markkurossi/riscv/emulator/linux"
	"github.com/markkurossi/riscv/hw"
	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/posix"
)

type Emulator struct {
	Verbose bool
	Ktrace  bool

	CPU *hw.CPU
	Mem *hw.MemoryX

	ProgBase    uint64
	ProgBaseEnd uint64

	Prog   *fileInfo
	Interp *fileInfo

	Kernel  *posix.Kernel
	Process *posix.Process
}

func New(ktrace bool) *Emulator {
	mem := new(hw.MemoryX)

	mem.MmapStart = 0x4000000000
	mem.MmapEnd = mem.MmapStart

	stack := hw.NewStack(0x7ffff000, 1<<20)
	mem.Add(stack)

	cpu := &hw.CPU{
		Mem: mem,
	}
	cpu.X[isa.Sp] = stack.End

	emu := &Emulator{
		Ktrace:      ktrace,
		CPU:         cpu,
		Mem:         mem,
		ProgBase:    0x400000,
		ProgBaseEnd: 0x400000,

		Kernel: &posix.Kernel{
			Ktrace: ktrace,
		},
	}

	cpu.Syscall = emu.Syscall

	return emu
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
	Phoff   uint64 // ELF header e_phoff, used to compute Phdr when PT_PHDR absent
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
	// Compute phoff by reading e_phoff from the raw ELF file header.
	// ELF64: e_phoff is at bytes 32-39, little-endian for RISC-V.
	{
		raw, err2 := os.Open(file)
		if err2 == nil {
			var buf [8]byte
			if _, err2 = raw.ReadAt(buf[:], 32); err2 == nil {
				info.Phoff = f.ByteOrder.Uint64(buf[:])
			}
			raw.Close()
		}
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
			vaddr := prog.Vaddr
			if info.Dynamic {
				vaddr += emu.ProgBase
			}
			end := vaddr + prog.Memsz + 4095
			end &= ^uint64(0xfff)

			headPad := vaddr & 0xfff
			vaddr &= ^uint64(0xfff)

			fmt.Printf(" @ Vaddr: %x-%x\n", vaddr, end)

			if end > emu.ProgBaseEnd {
				emu.ProgBaseEnd = end
			}
			if info.Base == 0 {
				info.Base = vaddr
			}

			data := make([]byte, end-vaddr)
			_, err = prog.ReadAt(data[headPad:headPad+prog.Filesz], 0)
			if err != nil {
				return nil, err
			}

			seg := &hw.Segment{
				Start: vaddr,
				End:   end,
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

	// If no PT_PHDR segment was present, compute the program header
	// address from the first load address plus the ELF header's
	// phoff. glibc requires a valid AT_PHDR to locate the program's
	// PT_DYNAMIC and .fini_array; AT_PHDR=0 causes it to register a
	// null atexit handler and crash on exit.
	if info.Phdr == 0 && info.Base != 0 {
		info.Phdr = info.Base + info.Phoff
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

	emu.Push(0)
	emu.Push(linux.AtNull)

	emu.Push(atRandom)
	emu.Push(linux.AtRandom)

	emu.Push(argvPtrs[0])
	emu.Push(linux.AtExecfn)

	emu.Push(4096)
	emu.Push(linux.AtPagesz)

	emu.Push(0x112d)
	emu.Push(linux.AtHwcap)

	emu.Push(100)
	emu.Push(linux.AtClktck)

	phdr := emu.Prog.Phdr
	if phdr == 0 {
		phdr = emu.Prog.Base + emu.Prog.Phoff
	}
	emu.Push(phdr)
	emu.Push(linux.AtPhdr)

	emu.Push(56)
	emu.Push(linux.AtPhent)

	emu.Push(emu.Prog.Phnum)
	emu.Push(linux.AtPhnum)

	if emu.Interp != nil {
		emu.Push(emu.Interp.Base)
		emu.Push(linux.AtBase)
	} else {
		emu.Push(0)
		emu.Push(linux.AtBase)
	}

	emu.Push(emu.Prog.Entry)
	emu.Push(linux.AtEntry)

	emu.Push(1000)
	emu.Push(linux.AtUID)

	emu.Push(1000)
	emu.Push(linux.AtEuid)

	emu.Push(1000)
	emu.Push(linux.AtGID)

	emu.Push(1000)
	emu.Push(linux.AtEgid)

	// AT_SECURE = 0 (not setuid)
	emu.Push(0)
	emu.Push(linux.AtSecure)

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

	seg, ofs, err := emu.Mem.Map(emu.CPU.X[isa.Sp], hw.AccessWrite, 1)
	if err != nil {
		return err
	}
	emu.Debugf("Stack:\n%s", hex.Dump(seg.Data[ofs:]))

	if emu.Interp != nil {
		emu.CPU.PC = emu.Interp.Entry
	} else {
		emu.CPU.PC = emu.Prog.Entry
	}

	// Clear argument registers a0-a7.
	for i := 0; i < 8; i++ {
		emu.CPU.X[isa.A0+isa.Register(i)] = 0
	}

	emu.Process = emu.Kernel.NewProcess(nil)
	emu.Process.CPU = emu.CPU
	emu.Process.FDs = []*os.File{
		os.Stdin,
		os.Stdout,
		os.Stderr,
	}
	emu.Process.CPU.PID = emu.Process.PID

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

func (emu *Emulator) Syscall(cpu *hw.CPU, id, a0, a1, a2, a3, a4, a5 uint64) (
	uint64, error) {

	return linux.Syscall(emu.Process, id, a0, a1, a2, a3, a4, a5)
}
