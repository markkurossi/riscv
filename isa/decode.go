//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package isa

import (
	"debug/elf"
	"encoding/binary"
	"fmt"
)

var (
	bo = binary.LittleEndian
)

func DecodeELF(file string) (*Program, error) {
	f, err := elf.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var text *elf.Section
	var textIdx elf.SectionIndex

	for idx, section := range f.Sections {
		if section.Type == elf.SHT_PROGBITS {
			text = section
			textIdx = elf.SectionIndex(idx)
			break
		}
	}
	if text == nil {
		return nil, fmt.Errorf(".text section not found")
	}
	fmt.Printf(".text.Type=%v[%d]\n", text.Type, text.Type)

	prog := NewProgram()

	symbols, err := f.Symbols()
	if err != nil {
		symbols, err = f.DynamicSymbols()
		if err != nil {
			return nil, err
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
		prog.AddFunction(sym.Name, sym.Value)
	}

	fmt.Printf("%016x <_start>:\n", f.Entry)
	prog.AddFunction("_start", f.Entry)

	data, err := text.Data()
	if err != nil {
		return nil, err
	}

	fmt.Printf(".text at 0x%x, size %d bytes\n", text.Addr, len(data))

	err = prog.Decode(data, text.Addr)
	if err != nil {
		return nil, err
	}

	return prog, nil
}

// Decode decodes RISC-V instructions from data and returns the
// decoded program.
func (prog *Program) Decode(data []byte, pc uint64) error {
	for len(data) > 0 {
		if len(data) < 2 {
			return fmt.Errorf("truncated data")
		}
		opcode := data[0]
		if opcode&0b11 != 0b11 {
			return fmt.Errorf("compressed instructions not supported yet")
		}

		// 32-bit (or longer) instructions.
		if len(data) < 4 {
			return fmt.Errorf("truncated >=32-bit instruction")
		}
		raw := bo.Uint32(data)

		f, ok := prog.symbols[pc]
		if ok {
			f.Offset = len(prog.Code)
		}

		group := Group(opcode & 0b1111111)

		instr := &Instr{
			Raw:   raw,
			Rd:    Register((raw >> 7) & 0b0011111),
			Func3: uint8((raw >> 12) & 0b0000111),
			Rs1:   Register((raw >> 15) & 0b0011111),
			Rs2:   Register((raw >> 20) & 0b0011111),
			Func7: uint8((raw >> 25) & 0b1111111),
		}

		column := (opcode >> 2) & 0b111
		switch column {
		case 0b101:
			instr.Imm = int32(raw) >> 12
			switch group {
			case GroupLUI:
				instr.Op = Lui
			}

		case 0b111:
			return fmt.Errorf("extended-length instructions not supported")
		}

		fmt.Printf("%8x:\t%08x\t%v\n", pc, instr.Raw, instr)

		data = data[4:]
		pc += 4
	}

	return nil
}
