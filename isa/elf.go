//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package isa

import (
	"debug/elf"
	"encoding/hex"
	"fmt"
)

func DecodeELF(file string) error {
	f, err := elf.Open(file)
	if err != nil {
		return err
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
	fmt.Printf(" - Entry     : %v\n", f.Entry)

	for idx, prog := range f.Progs {
		fmt.Printf("Prog %v\n", idx)
		fmt.Printf(" - Type : %v\n", prog.Type)
		fmt.Printf(" - Flags: %v\n", prog.Flags)
		fmt.Printf(" - Vaddr: %x\n", prog.Vaddr)
		fmt.Printf(" - Memsz: %x\n", prog.Memsz)
		fmt.Printf(" - Align: %x\n", prog.Align)

		if prog.Type == elf.PT_LOAD {
			vaddr := prog.Vaddr
			end := vaddr + prog.Memsz + 4095
			end &= ^uint64(0xfff)

			headPad := vaddr & 0xfff
			vaddr &= ^uint64(0xfff)

			data := make([]byte, end-vaddr)
			n, err := prog.ReadAt(data[headPad:headPad+prog.Filesz], 0)
			if err != nil {
				return err
			}

			limit := 256
			suffix := ""
			if n > limit {
				suffix = fmt.Sprintf("...%d bytes omitted...\n", n-limit)
				n = limit
			}
			fmt.Printf("%s%s", hex.Dump(data[headPad:headPad+uint64(n)]),
				suffix)
		}
	}

	var text *elf.Section
	var textIdx elf.SectionIndex

	for idx, section := range f.Sections {
		fmt.Printf("Section  %v\n", section.Name)
		fmt.Printf(" - Type: %v\n", section.Type)
		fmt.Printf(" - Addr: %x\n", section.Addr)
		fmt.Printf(" - Size: %x\n", section.Size)

		switch section.Name {
		case ".text":
			text = section
			textIdx = elf.SectionIndex(idx)

		case ".rodata":
			data, err := section.Data()
			if err != nil {
				return err
			}
			l := len(data)
			if l > 32 {
				l = 32
			}
			fmt.Printf("%s", hex.Dump(data[:l]))
		}
	}
	if text == nil {
		return fmt.Errorf(".text section not found")
	}
	fmt.Printf(".text.Type=%v[%d]\n", text.Type, text.Type)

	symbols, err := f.Symbols()
	if err != nil {
		symbols, err = f.DynamicSymbols()
		if err != nil {
			return err
		}
	}
	for _, sym := range symbols {
		if elf.ST_TYPE(sym.Info) != elf.STT_FUNC {
			continue
		}
		if sym.Section != textIdx {
			continue
		}
		fmt.Printf("%016x <%v>:\n", sym.Value, sym.Name)
	}

	fmt.Printf("%016x <_start>:\n", f.Entry)

	data, err := text.Data()
	if err != nil {
		return err
	}

	fmt.Printf(".text at 0x%x, size %d bytes\n", text.Addr, len(data))

	var lastDescOp Op
	pc := text.Addr

	for len(data) > 0 {
		var instr Instr
		var size uint64
		var raw uint32

		if data[0]&0b11 == 0b11 {
			if len(data) < 4 {
				return fmt.Errorf("truncated .text")
			}
			raw = bo.Uint32(data[:4])
			instr, err = Decode(raw)
			size = 4
		} else {
			if len(data) < 2 {
				return fmt.Errorf("truncated .text")
			}
			raw = uint32(bo.Uint16(data[:2]))
			instr, err = DecodeC(uint16(raw))
			size = 2
		}
		var line string
		if size == 4 {
			line = fmt.Sprintf("%8x:  %08x   %v", pc, raw, instr)
		} else {
			line = fmt.Sprintf("%8x:  %04x       %v", pc, raw, instr)
		}
		op, ok := Operands[instr.Op]
		if ok && len(op.Desc) > 0 && instr.Op != lastDescOp {
			lastDescOp = instr.Op

			for len(line) < 47 {
				line += " "
			}
			line += fmt.Sprintf("# %s", op.Desc)
		}
		fmt.Println(line)

		data = data[size:]
		pc += size
	}

	return nil
}
