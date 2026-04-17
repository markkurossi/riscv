//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package isa

import (
	"debug/elf"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
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

	for idx, prog := range f.Progs {
		fmt.Printf("Prog %v\n", idx)
		fmt.Printf(" - Type : %v\n", prog.Type)
		fmt.Printf(" - Flags: %v\n", prog.Flags)
		fmt.Printf(" - Vaddr: %x\n", prog.Vaddr)
		fmt.Printf(" - Memsz: %v\n", prog.Memsz)
		fmt.Printf(" - Align: %v\n", prog.Align)
	}

	var text *elf.Section
	var textIdx elf.SectionIndex

	for idx, section := range f.Sections {
		fmt.Printf("Section  %v\n", section.Name)
		fmt.Printf(" - Type: %v\n", section.Type)
		fmt.Printf(" - Addr: %x\n", section.Addr)
		fmt.Printf(" - Size: %v\n", section.Size)

		switch section.Name {
		case ".text":
			text = section
			textIdx = elf.SectionIndex(idx)

		case ".rodata":
			data, err := section.Data()
			if err != nil {
				return nil, err
			}
			fmt.Printf("%s", hex.Dump(data))
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

	_, _, err = Decode(data, text.Addr)
	if err != nil {
		return nil, err
	}

	return prog, nil
}

// Decode decodes RISC-V instructions from data and returns the
// decoded program.
func Decode(data []byte, pc uint64) (Instr, int, error) {
	var instr Instr

	if len(data) < 2 {
		return instr, 0, fmt.Errorf("truncated data")
	}
	opcode := data[0]
	if opcode&0b11 != 0b11 {
		if opcode == 0 && data[1] == 0 {
			return instr, 0, io.EOF
		}
		return instr, 0, fmt.Errorf("compressed instructions not supported yet")
	}

	// 32-bit (or longer) instructions.
	if len(data) < 4 {
		return instr, 0, fmt.Errorf("truncated >=32-bit instruction")
	}
	raw := bo.Uint32(data)

	group := Group(opcode & 0b1111111)

	instr.Raw = raw
	instr.Rd = Register((raw >> 7) & 0b0011111)
	instr.Funct3 = uint8((raw >> 12) & 0b0000111)
	instr.Rs1 = Register((raw >> 15) & 0b0011111)
	instr.Rs2 = Register((raw >> 20) & 0b0011111)
	instr.Funct7 = uint8((raw >> 25) & 0b1111111)

	switch group {
	case GroupAUIPC:
		instr.typeU()
		instr.Op = Auipc

	case GroupLUI:
		instr.typeU()
		instr.Op = Lui

	case GroupSTORE:
		instr.typeS()
		switch instr.Funct3 {
		case 0:
			instr.Op = Sb
		case 1:
			instr.Op = Sh
		case 2:
			instr.Op = Sw
		case 3:
			instr.Op = Sd
		default:
			return instr, 0, fmt.Errorf("invalid STORE instr %x", instr.Raw)
		}

	case GroupLOAD:
		instr.typeI()
		switch instr.Funct3 {
		case 0:
			instr.Op = Lb
		case 1:
			instr.Op = Lh
		case 2:
			instr.Op = Lw
		case 3:
			instr.Op = Ld
		case 4:
			instr.Op = Lbu
		case 5:
			instr.Op = Lhu
		case 6:
			instr.Op = Lwu
		}

	case GroupOPIMM:
		instr.typeI()
		switch instr.Funct3 {
		case 0:
			instr.Op = Addi
		case 1:
			instr.Op = Slli
		case 2:
			instr.Op = Slti
		case 3:
			instr.Op = Sltiu
		case 4:
			instr.Op = Xori
		case 5:
			switch instr.Funct7 {
			case 0:
				instr.Op = Srli
			case 32:
				instr.Op = Srai
				instr.Imm &= 0b11111
			default:
				return instr, 0, fmt.Errorf("GroupOPIMM: Funct3=%v, raw=%08x",
					instr.Funct3, raw)
			}
		case 6:
			instr.Op = Ori
		case 7:
			instr.Op = Andi
		}

	case GroupOPIMM32:
		instr.typeI()
		switch instr.Funct3 {
		case 0:
			instr.Op = Addiw
		case 1:
			instr.Op = Slliw
		case 5:
			switch instr.Funct7 {
			case 0:
				instr.Op = Srliw
			case 32:
				instr.Op = Sraiw
				instr.Imm &= 0b11111
			default:
				return instr, 0, fmt.Errorf("GroupOPIMM32: Func7=%v",
					instr.Funct7)
			}
		}

	case GroupSYSTEM:
		instr.typeI()
		switch instr.Funct3 {
		case 0:
			if instr.Imm == 0 {
				instr.Op = Ecall
			} else {
				instr.Op = Ebreak
			}
		}

	case GroupJAL:
		instr.typeJ()
		instr.Op = Jal

	case GroupJALR:
		instr.typeI()
		instr.Op = Jalr

	case GroupBRANCH:
		instr.typeB()
		switch instr.Funct3 {
		case 0:
			instr.Op = Beq
		case 1:
			instr.Op = Bne
		case 4:
			instr.Op = Blt
		case 5:
			instr.Op = Bge
		case 6:
			instr.Op = Bltu
		case 7:
			instr.Op = Bgeu
		}

	case GroupOP:
		switch instr.Funct7 {
		case 0:
			switch instr.Funct3 {
			case 0:
				instr.Op = Add
			case 1:
				instr.Op = Sll
			case 2:
				instr.Op = Slt
			case 3:
				instr.Op = Sltu
			case 4:
				instr.Op = Xor
			case 5:
				instr.Op = Srl
			case 6:
				instr.Op = Or
			case 7:
				instr.Op = And
			default:
				return instr, 0, fmt.Errorf("group %v: funct7=%v, raw=%x",
					group, instr.Funct7, raw)
			}

		case 32:
			switch instr.Funct3 {
			case 0:
				instr.Op = Sub
			case 5:
				instr.Op = Sra
			default:
				return instr, 0, fmt.Errorf("group %v: funct7=%v, raw=%x",
					group, instr.Funct7, raw)
			}

		default:
			return instr, 0, fmt.Errorf("group %v: funct7=%v, raw=%x",
				group, instr.Funct7, raw)
		}

	case GroupOP32:
		switch instr.Funct7 {
		case 0:
			switch instr.Funct3 {
			case 0:
				instr.Op = Addw
			case 1:
				instr.Op = Sllw
			case 5:
				instr.Op = Srlw
			default:
				return instr, 0, fmt.Errorf("group %v: funct7=%v, raw=%x",
					group, instr.Funct7, raw)
			}

		case 1:
			switch instr.Funct3 {
			case 0:
				instr.Op = Mulw
			case 4:
				instr.Op = Divw
			case 5:
				instr.Op = Divuw
			case 6:
				instr.Op = Remw
			case 7:
				instr.Op = Remuw
			default:
				return instr, 0, fmt.Errorf("group %v: funct7=%v, raw=%x",
					group, instr.Funct7, raw)
			}

		case 4:
			switch instr.Funct3 {
			case 0:
				instr.Op = AddUw
			case 2:
				instr.Op = Sh1addUw
			case 4:
				instr.Op = Sh2addUw
			case 6:
				instr.Op = Sh3addUw
			default:
				return instr, 0, fmt.Errorf("group %v: funct7=%v, raw=%x",
					group, instr.Funct7, raw)
			}

		case 32:
			switch instr.Funct3 {
			case 0:
				instr.Op = Subw
			case 5:
				instr.Op = Sraw
			default:
				return instr, 0, fmt.Errorf("group %v: funct7=%v, raw=%x",
					group, instr.Funct7, raw)
			}

		case 48:
			switch instr.Funct3 {
			case 1:
				instr.Op = Rolw
			case 5:
				instr.Op = Rorw
			default:
				return instr, 0, fmt.Errorf("group %v: funct7=%v, raw=%x",
					group, instr.Funct7, raw)
			}

		default:
			return instr, 0, fmt.Errorf("group %v: funct7=%v, raw=%x",
				group, instr.Funct7, raw)
		}

	default:
		if group>>2 == 0b111 {
			return instr, 0,
				fmt.Errorf("extended-length instructions not supported")
		}
		fmt.Printf("decode: group %v not implemented yet\n", group)
	}

	return instr, 4, nil
}
