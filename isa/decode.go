//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package isa

import (
	"encoding/binary"
	"fmt"
)

var (
	bo       = binary.LittleEndian
	rvcTable [65536]Instr
)

func init() {
	for i := 0; i < 65536; i++ {
		instr, err := DecodeC(uint16(i))
		if err == nil {
			rvcTable[i] = instr
		}
	}
}

func DecodeCFast(raw uint16) Instr {
	return rvcTable[raw]
}

var compressedRegisters = [8]Register{
	S0, S1, A0, A1, A2, A3, A4, A5,
}

func DecodeC(raw uint16) (Instr, error) {
	var instr Instr

	if raw&0b11 == 0b11 || raw == 0 {
		return instr, fmt.Errorf("illegal 16-bit instruction: raw=%04x", raw)
	}

	rds1 := Register(raw >> 7 & 0b11111)
	rs2 := Register(raw >> 2 & 0b11111)
	funct3 := raw >> 13 & 0b111

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
			return instr, fmt.Errorf("raw=%04x, Q=0, funct3=%03b", raw, funct3)
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
					return instr, fmt.Errorf("raw=%04x, Q1/100/11/%03b",
						raw, f3)
				}

			default:
				return instr,
					fmt.Errorf("raw=%04x, Q1, funct3=%03b, funct2=%02b",
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
			return instr,
				fmt.Errorf("raw=%04x, Q=1, funct3=%03b", raw, funct3)
		}

	case 2:
		switch funct3 {
		case 0b000:
			instr.Rd = rds1
			instr.Rs1 = rds1
			instr.Imm = int32(raw&0b1111100)>>2 |
				int32(raw&0b10000_00000000)>>7
			instr.Op = Slli

		case 0b001:
			instr.Imm = int32(raw&0b11100)<<4 |
				int32(raw&0b1100000)>>2 |
				int32(raw&0b10000_00000000)>>7
			instr.Rd = rds1
			instr.Rs1 = Sp
			instr.Op = Fld

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
					return instr, fmt.Errorf("raw=%04x", raw)
				}
			} else {
				if rds1 == 0 {
					if rs2 == 0 {
						instr.Op = Ebreak
					} else {
						return instr, fmt.Errorf("raw=%04x", raw)
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

		case 0b101:
			instr.Rs1 = Sp
			instr.Rs2 = rs2
			instr.Imm = int32(raw&0b11100_00000000)>>7 |
				int32(raw&0b11_10000000)>>1
			instr.Op = Fsd

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
			return instr, fmt.Errorf("raw=%04x, Q=2, funct3=%03b", raw, funct3)
		}

	default:
		return instr, fmt.Errorf("raw=%04x, Q=%v", raw, raw&0b11)
	}

	return instr, nil
}

// Decode decodes RISC-V instructions from data and returns the
// decoded program.
func Decode(raw uint32) (Instr, error) {
	var instr Instr

	if raw&0b11 != 0b11 {
		return instr, fmt.Errorf("illegal 32-bit instruction: %04x", raw)
	}

	opcode := uint8(raw)
	group := Group(opcode & 0b1111111)

	instr.Rd = Register(raw >> 7 & 0b0011111)
	instr.Rs1 = Register(raw >> 15 & 0b0011111)
	instr.Rs2 = Register(raw >> 20 & 0b0011111)

	funct3 := uint8(raw >> 12 & 0b0000111)
	funct7 := uint8(raw >> 25 & 0b1111111)

	switch group {
	case GroupAUIPC:
		instr.typeU(raw)
		instr.Op = Auipc

	case GroupLUI:
		instr.typeU(raw)
		instr.Op = Lui

	case GroupSTORE:
		instr.typeS(raw)
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
			return instr, fmt.Errorf("invalid STORE instr %x", raw)
		}

	case GroupLOAD:
		instr.typeI(raw)
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
			return instr, fmt.Errorf("invalid LOAD instr %x", raw)
		}

	case GroupOPIMM:
		instr.typeI(raw)
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
		instr.typeI(raw)
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
				return instr, fmt.Errorf("GroupOPIMM32: Func7=%v", funct7)
			}
		}

	case GroupSYSTEM:
		switch funct3 {
		case 0:
			if raw>>25 == 0b1001 {
				instr.Op = SfenceVMA
			} else {
				// Trap/return.
				switch raw >> 20 {
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
					return instr,
						fmt.Errorf("invalid SYSTEM trap/return: raw=%08x", raw)
				}
			}

			// CSR mappings.
		case 1:
			instr.Op = Csrrw
			// Imm is csr.
			instr.Imm = int32(raw >> 20 & 0b1111_11111111)
		case 2:
			instr.Op = Csrrs
			// Imm is csr.
			instr.Imm = int32(raw >> 20 & 0b1111_11111111)
		case 3:
			instr.Op = Csrrc
			// Imm is csr.
			instr.Imm = int32(raw >> 20 & 0b1111_11111111)
		case 5:
			instr.Op = Csrrwi
			// Rs1 is zimm
			// Imm is csr
			instr.Imm = int32(raw >> 20 & 0b1111_11111111)
		case 6:
			instr.Op = Csrrsi
			// Rs1 is zimm
			// Imm is csr
			instr.Imm = int32(raw >> 20 & 0b1111_11111111)
		case 7:
			instr.Op = Csrrci
			// Rs1 is zimm
			// Imm is csr
			instr.Imm = int32(raw >> 20 & 0b1111_11111111)

		default:
			return instr, fmt.Errorf("invalid SYSTEM: raw=%08x", raw)
		}

	case GroupJAL:
		instr.typeJ(raw)
		instr.Op = Jal

	case GroupJALR:
		instr.typeI(raw)
		instr.Op = Jalr

	case GroupBRANCH:
		instr.typeB(raw)
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
				return instr,
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
				return instr,
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
				return instr,
					fmt.Errorf("invalid group OP funct3: %v, raw=%08x",
						funct3, raw)
			}

			// Zba (Address Generation Instructions) extension.

		case 0b0010000:
			switch funct3 {
			case 0b010:
				instr.Op = Sh1add
			case 0b100:
				instr.Op = Sh2add
			case 0b110:
				instr.Op = Sh3add
			default:
				return instr,
					fmt.Errorf("invalid group OP Zba funct3: %03b, raw=%08x",
						funct3, raw)
			}

		default:
			return instr, fmt.Errorf("group OP funct7: %v, raw=%08x",
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
				return instr, fmt.Errorf("group %v: funct7=%v, raw=%x",
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
				return instr, fmt.Errorf("group %v: funct7=%v, raw=%x",
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
				return instr, fmt.Errorf("group %v: funct7=%v, raw=%x",
					group, funct7, raw)
			}

		case 32:
			switch funct3 {
			case 0:
				instr.Op = Subw
			case 5:
				instr.Op = Sraw
			default:
				return instr, fmt.Errorf("group %v: funct7=%v, raw=%x",
					group, funct7, raw)
			}

		case 48:
			switch funct3 {
			case 1:
				instr.Op = Rolw
			case 5:
				instr.Op = Rorw
			default:
				return instr, fmt.Errorf("group %v: funct7=%v, raw=%x",
					group, funct7, raw)
			}

		default:
			return instr, fmt.Errorf("group %v: funct7=%v, raw=%x",
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
				return instr, fmt.Errorf("AMO/%05b/%03b: raw=%08x",
					funct7, funct3, raw)
			}

		case 0b00001:
			switch funct3 {
			case 2:
				instr.Op = AmoswapW
			case 3:
				instr.Op = AmoswapD
			default:
				return instr, fmt.Errorf("AMO/%05b/%03b: raw=%08x",
					funct7, funct3, raw)
			}

		case 0b00010:
			switch funct3 {
			case 2:
				instr.Op = LrW
			case 3:
				instr.Op = LrD
			default:
				return instr, fmt.Errorf("AMO/%05b/%03b: raw=%08x",
					funct7, funct3, raw)
			}

		case 0b00011:
			switch funct3 {
			case 2:
				instr.Op = ScW
			case 3:
				instr.Op = ScD
			default:
				return instr, fmt.Errorf("AMO/%05b/%03b: raw=%08x",
					funct7, funct3, raw)
			}

		case 0b00100:
			switch funct3 {
			case 2:
				instr.Op = AmoxorW
			case 3:
				instr.Op = AmoxorD
			default:
				return instr, fmt.Errorf("AMO/%05b/%03b: raw=%08x",
					funct7, funct3, raw)
			}

		case 0b01000:
			switch funct3 {
			case 2:
				instr.Op = AmoorW
			case 3:
				instr.Op = AmoorD
			default:
				return instr, fmt.Errorf("AMO/%05b/%03b: raw=%08x",
					funct7, funct3, raw)
			}

		case 0b01100:
			switch funct3 {
			case 2:
				instr.Op = AmoandW
			case 3:
				instr.Op = AmoandD
			default:
				return instr, fmt.Errorf("AMO/%05b/%03b: raw=%08x",
					funct7, funct3, raw)
			}

		case 0b11100:
			switch funct3 {
			case 2:
				instr.Op = AmomaxuW
			case 3:
				instr.Op = AmomaxuD
			default:
				return instr, fmt.Errorf("AMO/%05b/%03b: raw=%08x",
					funct7, funct3, raw)
			}

		default:
			return instr, fmt.Errorf("AMO/%05b: raw=%08x", funct5, raw)
		}

	case GroupLOADFP:
		switch funct3 {
		case 0b010:
			instr.Imm = int32(raw) >> 20
			instr.Op = Flw

		case 0b011:
			instr.Imm = int32(raw) >> 20
			instr.Op = Fld

			// Vector loads.
		case 0b000:
			instr.Imm = int32(raw >> 25)
			instr.Op = Vle8V

		case 0b101:
			instr.Imm = int32(raw >> 25)
			instr.Op = Vle16V

		case 0b110:
			instr.Imm = int32(raw >> 25)
			instr.Op = Vle32V

		case 0b111:
			instr.Imm = int32(raw >> 25)
			instr.Op = Vle64V

		default:
			return instr, fmt.Errorf("%v/%03b: raw=%08x", group, funct3, raw)
		}

	case GroupSTOREFP:
		switch funct3 {
		case 0b010:
			instr.Imm = int32(raw&0b1111_10000000)>>7 |
				int32(raw&0b11111110_00000000_00000000_00000000)>>20
			instr.Op = Fsw

		case 0b011:
			instr.Imm = int32(raw&0b1111_10000000)>>7 |
				int32(raw&0b11111110_00000000_00000000_00000000)>>20
			instr.Op = Fsd

			// Vector stores.

		case 0b000:
			instr.Imm = int32(raw >> 25)
			instr.Op = Vse8V

		case 0b101:
			instr.Imm = int32(raw >> 25)
			instr.Op = Vse16V

		case 0b110:
			instr.Imm = int32(raw >> 25)
			instr.Op = Vse32V

		case 0b111:
			instr.Imm = int32(raw >> 25)
			instr.Op = Vse64V

		default:
			return instr, fmt.Errorf("STORE-FP: funct3=%03b", funct3)
		}

	case GroupOPFP:
		switch funct7 {
		case 0b0000000:
			instr.Op = FaddS
		case 0b0000001:
			instr.Op = FaddD
		case 0b0000100:
			instr.Op = FsubS
		case 0b0000101:
			instr.Op = FsubD
		case 0b0001000:
			instr.Op = FmulS
		case 0b0001001:
			instr.Op = FmulD
		case 0b0001100:
			instr.Op = FdivS
		case 0b0001101:
			instr.Op = FdivD
		case 0b1010000:
			instr.Op = FeqS
		case 0b1010001:
			switch funct3 {
			case 0b000:
				instr.Op = FleD
			case 0b001:
				instr.Op = FltD
			case 0b010:
				instr.Op = FeqD
			default:
				return instr, fmt.Errorf("OP-FP: funct7=%07b, funct3=%03b",
					funct7, funct3)
			}

		case 0b1100001:
			funct5 := raw >> 20 & 0b11111
			switch funct5 {
			case 0b00000:
				instr.Op = FcvtWD
			case 0b00001:
				instr.Op = FcvtWUD
			case 0b00010:
				instr.Op = FcvtLD
			default:
				return instr, fmt.Errorf("OP-FP: funct7=%07b, funct5=%05b",
					funct7, funct5)
			}

		case 0b1100000:
			instr.Op = FcvtLUS

		case 0b1101001:
			funct5 := raw >> 20 & 0b11111
			switch funct5 {
			case 0b00000:
				instr.Op = FcvtDW
			case 0b00001:
				instr.Op = FcvtDWU
			case 0b00010:
				instr.Op = FcvtDL
			case 0b00011:
				instr.Op = FcvtDLU
			default:
				return instr, fmt.Errorf("OP-FP: funct7=%07b, funct5=%05b",
					funct7, funct5)
			}

		case 0b1110001:
			switch funct3 {
			case 0b000:
				instr.Op = FmvXD
			default:
				return instr, fmt.Errorf("OP-FP: funct7=%07b, funct3=%03b",
					funct7, funct3)
			}

		case 0b1111000:
			instr.Op = FmvWX
		case 0b1111001:
			instr.Op = FmvDX

		case 0b0010001:
			instr.Op = FsgnjD

		case 0b0100000:
			instr.Op = FcvtSD

		case 0b0100001:
			instr.Op = FcvtDS

		default:
			return instr, fmt.Errorf("OP-FP: funct7=%07b, raw=%08x",
				funct7, raw)
		}

	case GroupMADD:
		instr.Imm = int32(funct7 >> 2) // rs3
		switch funct7 & 0b11 {
		case 0b00:
			instr.Op = FmaddS
		case 0b01:
			instr.Op = FmaddD
		default:
			return instr, fmt.Errorf("MADD: funct2=%02b", funct7&0b11)
		}

	case GroupOPV:
		switch funct3 {
		case 0b100:
			switch funct7 >> 1 {
			case 0b010111:
				instr.Imm = int32(funct7 & 0b1) // Store bit 0: VM
				instr.Op = VmvVX

			default:
				return instr, fmt.Errorf("%v: funct3=%03b, funct6=%06b",
					group, funct3, funct7>>1)
			}

		case 0b111:
			if raw&(1<<31) == 0 {
				instr.Imm = int32((raw >> 20) & 0b111_11111111)
				instr.Op = Vsetvli
			} else if raw&(1<<30) != 0 {
				instr.Imm = int32((raw >> 20) & 0b011_11111111)
				// Rs1 is uimm[4:0]
				instr.Op = Vsetivli
			} else {
				instr.Op = Vsetvl
			}
		default:
			return instr, fmt.Errorf("%v: funct3=%03b, raw=%08x",
				group, funct3, raw)
		}

	default:
		if group>>2 == 0b111 {
			return instr,
				fmt.Errorf("extended-length instructions not supported")
		}
		return instr, fmt.Errorf("group %v not implemented yet: raw=%08x",
			group, raw)
	}

	return instr, nil
}
