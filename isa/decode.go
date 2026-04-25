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
)

var (
	bo = binary.LittleEndian
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

		data := make([]byte, prog.Memsz)
		n, err := prog.ReadAt(data, 0)
		if err != nil {
			return err
		}
		limit := 256
		suffix := ""
		if n > limit {
			suffix = fmt.Sprintf("...%d bytes omitted...\n", n-limit)
			n = limit
		}
		fmt.Printf("%s%s", hex.Dump(data[:n]), suffix)
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

	_, _, err = Decode(data)
	if err != nil {
		return err
	}

	return nil
}

var compressedRegisters = [8]Register{
	S0, S1, A0, A1, A2, A3, A4, A5,
}

// Decode decodes RISC-V instructions from data and returns the
// decoded program.
func Decode(data []byte) (Instr, int, error) {
	var instr Instr

	if len(data) < 2 {
		return instr, 0, fmt.Errorf("truncated data")
	}
	opcode := data[0]
	if opcode&0b11 != 0b11 {
		if opcode == 0 && data[1] == 0 {
			return instr, 0, fmt.Errorf("illegal instruction: raw=000")
		}
		raw := bo.Uint16(data)
		rds1 := Register(raw >> 7 & 0b11111)
		rs2 := Register(raw >> 2 & 0b11111)
		funct3 := raw >> 13 & 0b111

		instr.Raw = uint32(raw)

		// Switch by quadrants.
		switch raw & 0b11 {
		case 0:
			switch funct3 {
			case 0b000:
				instr.Imm = int32(raw&0b1000000)>>4 |
					int32(raw&0b100000)>>2 |
					int32(raw&0b11000_00000000)>>7 |
					int32(raw&0b111_10000000)>>1
				instr.Rd = compressedRegisters[raw>>2&0b111]
				instr.Rs1 = Sp
				instr.Op = Addi

			case 0b001:
				instr.Rd = compressedRegisters[raw>>2&0b111]
				instr.Rs1 = compressedRegisters[raw>>7&0b111]
				instr.Imm = int32(raw&0b11100_00000000)>>7 |
					int32(raw&0b1100000)<<1
				instr.Op = Fld

			case 0b010:
				instr.Rd = compressedRegisters[raw>>2&0b111]
				instr.Rs1 = compressedRegisters[raw>>7&0b111]
				instr.Imm = int32(raw&0b1000000)>>4 |
					int32(raw&0b11100_00000000)>>7 |
					int32(raw&0b100000)<<1
				instr.Op = Lw

			case 0b011:
				instr.Rd = compressedRegisters[raw>>2&0b111]
				instr.Rs1 = compressedRegisters[raw>>7&0b111]
				instr.Imm = int32(raw&0b11100_00000000)>>7 |
					int32(raw&0b1100000)<<1
				instr.Op = Ld

			case 0b101:
				instr.Rs2 = compressedRegisters[raw>>2&0b111]
				instr.Rs1 = compressedRegisters[raw>>7&0b111]
				instr.Imm = int32(raw&0b11100_00000000)>>7 |
					int32(raw&0b1100000)<<1
				instr.Op = Fsd

			case 0b110:
				instr.Rs2 = compressedRegisters[raw>>2&0b111]
				instr.Rs1 = compressedRegisters[raw>>7&0b111]
				instr.Imm = int32(raw&0b1000000)>>4 |
					int32(raw&0b11100_00000000)>>7 |
					int32(raw&0b100000)<<1
				instr.Op = Sw

			case 0b111:
				instr.Rs2 = compressedRegisters[raw>>2&0b111]
				instr.Rs1 = compressedRegisters[raw>>7&0b111]
				instr.Imm = int32(raw&0b11100_00000000)>>7 |
					int32(raw&0b1100000)<<1
				instr.Op = Sd

			default:
				return instr, 0,
					fmt.Errorf("compressed: raw=%04x, Q=0, funct3=%03b",
						raw, funct3)
			}

		case 1:
			switch funct3 {
			case 0b000:
				instr.Rd = rds1
				instr.Rs1 = rds1
				instr.Imm = int32(raw&0b1111100)>>2 |
					int32(raw&0b10000_00000000)>>7
				if instr.Imm&0b100000 != 0 {
					// XXX change all sign extends to use this pattern
					instr.Imm |= ^int32(0b111111)
				}
				instr.Op = Addi

			case 0b001:
				instr.Rd = rds1
				instr.Rs1 = rds1
				instr.Imm = int32(raw&0b1111100)>>2 |
					int32(raw&0b10000_00000000)>>7
				if instr.Imm&0b100000 != 0 {
					instr.Imm |= ^int32(0b111111)

				}
				instr.Op = Addiw

			case 0b010:
				instr.Rd = rds1
				instr.Rs1 = Zero
				instr.Imm = int32(raw&0b1111100)>>2 |
					int32(raw&0b10000_00000000)>>7
				if instr.Imm&0b100000 != 0 {
					instr.Imm |= ^int32(0b111111)
				}
				instr.Op = Addi

			case 0b011:
				instr.Rd = rds1
				if rds1 == 2 {
					instr.Imm = int32(raw&0b1000000)>>2 |
						int32(raw&0b100)<<3 |
						int32(raw&0b100000)<<1 |
						int32(raw&0b11000)<<4 |
						int32(raw&0b10000_00000000)>>3
					if instr.Imm&0b1000000000 != 0 {
						instr.Imm |= int32(-1) << 10
					}
					instr.Rs1 = Sp
					instr.Op = Addi
				} else {
					instr.Rd = rds1
					instr.Imm = int32(raw&0b1111100)<<10 |
						int32(raw&0b10000_00000000)<<5
					if instr.Imm&0b10_00000000_00000000 != 0 {
						instr.Imm |= int32(-1) << 18
					}
					instr.Op = Lui
				}

			case 0b100:
				funct2 := raw >> 10 & 0b11
				switch funct2 {
				case 0b00:
					instr.Rd = compressedRegisters[raw>>7&0b111]
					instr.Rs1 = instr.Rd
					instr.Imm = int32(raw&0b1111100)>>2 |
						int32(raw&0b10000_00000000)>>7
					instr.Op = Srli

				case 0b01:
					instr.Rd = compressedRegisters[raw>>7&0b111]
					instr.Rs1 = instr.Rd
					instr.Imm = int32(raw&0b1111100)>>2 |
						int32(raw&0b10000_00000000)>>7
					instr.Op = Srai

				case 0b10:
					instr.Rd = compressedRegisters[raw>>7&0b111]
					instr.Rs1 = instr.Rd
					instr.Imm = int32(raw&0b1111100)>>2 |
						int32(raw&0b10000_00000000)>>7
					if instr.Imm&0b100000 != 0 {
						instr.Imm |= int32(-1) << 6
					}
					instr.Op = Andi

				case 0b11:
					f3 := (raw >> 5 & 0b11) | (raw >> 10 & 0b100)
					switch f3 {
					case 0b000:
						instr.Rd = compressedRegisters[raw>>7&0b111]
						instr.Rs1 = instr.Rd
						instr.Rs2 = compressedRegisters[raw>>2&0b111]
						instr.Op = Sub

					case 0b001:
						instr.Rd = compressedRegisters[raw>>7&0b111]
						instr.Rs1 = instr.Rd
						instr.Rs2 = compressedRegisters[raw>>2&0b111]
						instr.Op = Xor

					case 0b010:
						instr.Rd = compressedRegisters[raw>>7&0b111]
						instr.Rs1 = instr.Rd
						instr.Rs2 = compressedRegisters[raw>>2&0b111]
						instr.Op = Or

					case 0b011:
						instr.Rd = compressedRegisters[raw>>7&0b111]
						instr.Rs1 = instr.Rd
						instr.Rs2 = compressedRegisters[raw>>2&0b111]
						instr.Op = And

					case 0b101:
						instr.Rd = compressedRegisters[raw>>7&0b111]
						instr.Rs1 = instr.Rd
						instr.Rs2 = compressedRegisters[raw>>2&0b111]
						instr.Op = Addw

					case 0b100:
						instr.Rd = compressedRegisters[raw>>7&0b111]
						instr.Rs1 = instr.Rd
						instr.Rs2 = compressedRegisters[raw>>2&0b111]
						instr.Op = Subw

					default:
						return instr, 0, fmt.Errorf("raw=%04x, Q1/100/11/%03b",
							raw, f3)
					}

				default:
					return instr, 0,
						fmt.Errorf("compressed: raw=%04x, Q1, funct3=%03b, funct2=%02b",
							raw, funct3, funct2)
				}

			case 0b101:
				instr.Imm = int32(raw&0b111000)>>2 |
					int32(raw&0b1000_00000000)>>7 |
					int32(raw&0b100)<<3 |
					int32(raw&0b10000000)>>1 |
					int32(raw&0b1000000)<<1 |
					int32(raw&0b110_00000000)>>1 |
					int32(raw&0b1_00000000)<<2 |
					int32(raw&0b10000_00000000)>>1
				if instr.Imm&0b1000_00000000 != 0 {
					instr.Imm |= ^int32(0b1111_11111111)
				}
				instr.Op = Jal

			case 0b110, 0b111:
				instr.Rs1 = compressedRegisters[raw>>7&0b111]
				instr.Imm = int32(raw&0b11000)>>2 |
					int32(raw&0b1100_00000000)>>7 |
					int32(raw&0b100)<<3 |
					int32(raw&0b1100000)<<1 |
					int32(raw&0b10000_00000000)>>4
				if instr.Imm&0b1_00000000 != 0 {
					instr.Imm |= ^int32(0b11111111)
				}
				if funct3 == 0b110 {
					instr.Op = Beq
				} else {
					instr.Op = Bne
				}

			default:
				return instr, 0,
					fmt.Errorf("compressed: raw=%04x, Q=1, funct3=%03b",
						raw, funct3)
			}

		case 2:
			switch funct3 {
			case 0b000:
				instr.Rd = rds1
				instr.Rs1 = rds1
				instr.Imm = int32(raw&0b1111100)>>2 |
					int32(raw&0b10000_00000000)>>7
				instr.Op = Slli

			case 0b010:
				instr.Imm = int32(raw&0b1110000)>>2 |
					int32(raw&0b10000_00000000)>>7 |
					int32(raw&0b1100)<<4
				instr.Rd = rds1
				instr.Rs1 = Sp
				instr.Op = Lw

			case 0b011:
				instr.Imm = int32(raw&0b11100)<<4 |
					int32(raw&0b1100000)>>2 |
					int32(raw&0b10000_00000000)>>7
				instr.Rd = rds1
				instr.Rs1 = Sp
				instr.Op = Ld

			case 0b100:
				if raw&0b10000_00000000 == 0 {
					if rds1 != 0 {
						if rs2 == 0 {
							instr.Rs1 = rds1
							instr.Op = Jalr
						} else {
							instr.Rd = rds1
							instr.Rs2 = rs2
							instr.Op = Add
						}
					} else {
						return instr, 0, fmt.Errorf("compressed: raw=%04x", raw)
					}
				} else {
					if rds1 == 0 {
						if rs2 == 0 {
							instr.Op = Ebreak
						} else {
							return instr, 0, fmt.Errorf("compressed: raw=%04x",
								raw)
						}
					} else {
						if rs2 == 0 {
							instr.Rd = Ra
							instr.Rs1 = rds1
							instr.Op = Jalr
						} else {
							instr.Rd = rds1
							instr.Rs1 = rds1
							instr.Rs2 = rs2
							instr.Op = Add
						}
					}
				}

			case 0b110:
				instr.Rs1 = Sp
				instr.Rs2 = rs2
				instr.Imm = int32(raw&0b11110_00000000)>>7 |
					int32(raw&0b01_10000000)>>1
				instr.Op = Sw

			case 0b111:
				instr.Rs1 = Sp
				instr.Rs2 = rs2
				instr.Imm = int32(raw&0b11100_00000000)>>7 |
					int32(raw&0b11_10000000)>>1
				instr.Op = Sd

			default:
				return instr, 0,
					fmt.Errorf("compressed: raw=%04x, Q=2, funct3=%03b",
						raw, funct3)
			}

		default:
			return instr, 0,
				fmt.Errorf("compressed: raw=%04x, Q=%v", raw, raw&0b11)
		}
		return instr, 2, nil
	}

	// 32-bit (or longer) instructions.
	if len(data) < 4 {
		return instr, 0, fmt.Errorf("truncated >=32-bit instruction")
	}
	raw := bo.Uint32(data)

	group := Group(opcode & 0b1111111)

	instr.Raw = raw
	instr.Rd = Register(raw >> 7 & 0b0011111)
	instr.Rs1 = Register(raw >> 15 & 0b0011111)
	instr.Rs2 = Register(raw >> 20 & 0b0011111)

	funct3 := uint8(raw >> 12 & 0b0000111)
	funct7 := uint8(raw >> 25 & 0b1111111)

	switch group {
	case GroupAUIPC:
		instr.typeU()
		instr.Op = Auipc

	case GroupLUI:
		instr.typeU()
		instr.Op = Lui

	case GroupSTORE:
		instr.typeS()
		switch funct3 {
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
		switch funct3 {
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
		default:
			return instr, 0, fmt.Errorf("invalid LOAD instr %x", instr.Raw)
		}

	case GroupOPIMM:
		instr.typeI()
		switch funct3 {
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
			if funct7&0b100000 == 0 {
				instr.Op = Srli
			} else {
				instr.Op = Srai
			}
			instr.Imm &= 0b111111
		case 6:
			instr.Op = Ori
		case 7:
			instr.Op = Andi
		}

	case GroupOPIMM32:
		instr.typeI()
		switch funct3 {
		case 0:
			instr.Op = Addiw
		case 1:
			instr.Op = Slliw
		case 5:
			switch funct7 {
			case 0:
				instr.Op = Srliw
			case 32:
				instr.Op = Sraiw
				instr.Imm &= 0b111111
			default:
				return instr, 0, fmt.Errorf("GroupOPIMM32: Func7=%v",
					funct7)
			}
		}

	case GroupSYSTEM:
		instr.typeI()
		switch funct3 {
		case 0:
			// Trap/return.
			switch instr.Imm {
			case 0x0:
				instr.Op = Ecall
			case 0x1:
				instr.Op = Ebreak
			case 0x102:
				instr.Op = Sret
			case 0x105:
				instr.Op = Wfi
			case 0x302:
				instr.Op = Mret
			default:
				return instr, 0,
					fmt.Errorf("invalid SYSTEM trap/return: raw=%08x", raw)
			}

			// CSR mappings.
		case 1:
			instr.Op = Csrrw
		case 2:
			instr.Op = Csrrs
		case 3:
			instr.Op = Csrrc
		case 5:
			instr.Op = Csrrwi
		case 6:
			instr.Op = Csrrsi
		case 7:
			instr.Op = Csrrci

		default:
			return instr, 0, fmt.Errorf("invalid SYSTEM: raw=%08x", raw)
		}

	case GroupJAL:
		instr.typeJ()
		instr.Op = Jal

	case GroupJALR:
		instr.typeI()
		instr.Op = Jalr

	case GroupBRANCH:
		instr.typeB()
		switch funct3 {
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
		switch funct7 {
		case 0:
			switch funct3 {
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
				return instr, 0,
					fmt.Errorf("invalid group OP funct3: %v, raw=%08x",
						funct3, raw)
			}

			// The 'M' Extension (Multiply/Divide)
		case 1:
			switch funct3 {
			case 0:
				instr.Op = Mul
			case 1:
				instr.Op = Mulh
			case 2:
				instr.Op = Mulhsu
			case 3:
				instr.Op = Mulhu
			case 4:
				instr.Op = Div
			case 5:
				instr.Op = Divu
			case 6:
				instr.Op = Rem
			case 7:
				instr.Op = Remu
			default:
				return instr, 0,
					fmt.Errorf("invalid group OP M-ext funct3: %v, raw=%08x",
						funct3, raw)
			}

		case 32:
			switch funct3 {
			case 0:
				instr.Op = Sub
			case 5:
				instr.Op = Sra
			default:
				return instr, 0,
					fmt.Errorf("invalid group OP funct3: %v, raw=%08x",
						funct3, raw)
			}

		default:
			return instr, 0, fmt.Errorf("group OP funct7: %v, raw=%08x",
				funct7, raw)
		}

	case GroupOP32:
		switch funct7 {
		case 0:
			switch funct3 {
			case 0:
				instr.Op = Addw
			case 1:
				instr.Op = Sllw
			case 5:
				instr.Op = Srlw
			default:
				return instr, 0, fmt.Errorf("group %v: funct7=%v, raw=%x",
					group, funct7, raw)
			}

		case 1:
			switch funct3 {
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
					group, funct7, raw)
			}

		case 4:
			switch funct3 {
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
					group, funct7, raw)
			}

		case 32:
			switch funct3 {
			case 0:
				instr.Op = Subw
			case 5:
				instr.Op = Sraw
			default:
				return instr, 0, fmt.Errorf("group %v: funct7=%v, raw=%x",
					group, funct7, raw)
			}

		case 48:
			switch funct3 {
			case 1:
				instr.Op = Rolw
			case 5:
				instr.Op = Rorw
			default:
				return instr, 0, fmt.Errorf("group %v: funct7=%v, raw=%x",
					group, funct7, raw)
			}

		default:
			return instr, 0, fmt.Errorf("group %v: funct7=%v, raw=%x",
				group, funct7, raw)
		}

	case GroupMISCMEM:
		// XXX
		instr.Op = Fence

	case GroupAMO:
		funct5 := raw >> 27 & 0b11111
		// aq := raw >> 26 & 0b1
		// rl := raw >> 25 & 0b1
		// funct3 is width: .W/.D
		switch funct5 {
		case 0b00000:
			switch funct3 {
			case 2:
				instr.Op = AmoaddW
			case 3:
				instr.Op = AmoaddD
			default:
				return instr, 0, fmt.Errorf("AMO/%05b/%03b: raw=%08x",
					funct7, funct3, raw)
			}

		case 0b00001:
			switch funct3 {
			case 2:
				instr.Op = AmoswapW
			case 3:
				instr.Op = AmoswapD
			default:
				return instr, 0, fmt.Errorf("AMO/%05b/%03b: raw=%08x",
					funct7, funct3, raw)
			}

		case 0b00010:
			switch funct3 {
			case 2:
				instr.Op = LrW
			case 3:
				instr.Op = LrD
			default:
				return instr, 0, fmt.Errorf("AMO/%05b/%03b: raw=%08x",
					funct7, funct3, raw)
			}

		case 0b00011:
			switch funct3 {
			case 2:
				instr.Op = ScW
			case 3:
				instr.Op = ScD
			default:
				return instr, 0, fmt.Errorf("AMO/%05b/%03b: raw=%08x",
					funct7, funct3, raw)
			}

		default:
			return instr, 0, fmt.Errorf("AMO/%05b: raw=%08x", funct5, raw)
		}

	case GroupLOADFP:
		switch funct3 {
		case 0b011:
			instr.Imm = int32(raw>>20) & 0b1111_11111111
			instr.Op = Fld

		default:
			return instr, 0, fmt.Errorf("%v/%03b: raw=%08x", group, funct3, raw)
		}

	case GroupSTOREFP:
		instr.Imm = int32(raw&0b1111_10000000)>>7 |
			int32(raw&0b11111110_00000000_00000000_00000000)>>20
		switch funct3 {
		case 0b010:
			instr.Op = Fsw
		case 0b011:
			instr.Op = Fsd
		default:
			return instr, 0, fmt.Errorf("STORE-FP: funct3=%03b", funct3)
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
