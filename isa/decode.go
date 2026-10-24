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

// DecodeCFast implements a fast O(1) decoding of the compressed
// instruction raw. The fast decoding uses a pre-computed
// 65536-element array that maps raw instruction values into the
// decoded instructions.
func DecodeCFast(raw uint16) Instr {
	return rvcTable[raw]
}

var compressedRegisters = [8]Register{
	S0, S1, A0, A1, A2, A3, A4, A5,
}

// DecodeC decodes the compressed instruction raw.
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
	case 0b00: // Quadrant 0
		switch funct3 {
		case 0b000: // C.ADDI4SPN
			instr.Imm = int32(raw&0b1000000)>>4 |
				int32(raw&0b100000)>>2 |
				int32(raw&0b11000_00000000)>>7 |
				int32(raw&0b111_10000000)>>1
			instr.Rd = compressedRegisters[raw>>2&0b111]
			instr.Rs1 = Sp
			instr.Op = Addi

		case 0b001: // C.FLD
			instr.Rd = compressedRegisters[raw>>2&0b111]
			instr.Rs1 = compressedRegisters[raw>>7&0b111]
			instr.Imm = int32(raw&0b11100_00000000)>>7 |
				int32(raw&0b1100000)<<1
			instr.Op = Fld

		case 0b010: // C.LW
			instr.Rd = compressedRegisters[raw>>2&0b111]
			instr.Rs1 = compressedRegisters[raw>>7&0b111]
			instr.Imm = int32(raw&0b1000000)>>4 |
				int32(raw&0b11100_00000000)>>7 |
				int32(raw&0b100000)<<1
			instr.Op = Lw

		case 0b011: // C.LD
			instr.Rd = compressedRegisters[raw>>2&0b111]
			instr.Rs1 = compressedRegisters[raw>>7&0b111]
			instr.Imm = int32(raw&0b11100_00000000)>>7 |
				int32(raw&0b1100000)<<1
			instr.Op = Ld

		case 0b100:
			// "Zc*" Extension for Code Size Reduction.
			//
			// At this point, we have resolved quadrant Q0 [1:0] and
			// funct3 [15:13].
			f3 := raw >> 10 & 0b111
			switch f3 {
			case 0b000: // C.LBU
				instr.Rd = compressedRegisters[raw>>2&0b111]
				instr.Rs1 = compressedRegisters[raw>>7&0b111]
				instr.Imm = int32(raw>>6&0b1) | int32(raw>>4&0b10)
				instr.Op = Lbu

			case 0b001: // C.LHU / C.LH
				instr.Rd = compressedRegisters[raw>>2&0b111]
				instr.Rs1 = compressedRegisters[raw>>7&0b111]
				instr.Imm = int32(raw >> 4 & 0b10)
				if raw&0b1000000 == 0 {
					instr.Op = Lhu
				} else {
					instr.Op = Lh
				}

			case 0b010: // C.SB
				instr.Rs1 = compressedRegisters[raw>>7&0b111]
				instr.Rs2 = compressedRegisters[raw>>2&0b111]
				instr.Imm = int32(raw>>6&0b1) | int32(raw>>4&0b10)
				instr.Op = Sb

			case 0b011: // C.SH
				if raw&0b1000000 == 0 {
					instr.Rs1 = compressedRegisters[raw>>7&0b111]
					instr.Rs2 = compressedRegisters[raw>>2&0b111]
					instr.Imm = int32(raw >> 4 & 0b10)
					instr.Op = Sh
				} else {
					return instr, fmt.Errorf("zc*: Q0/%03b/%03b, raw=%04x",
						funct3, f3, raw)
				}

			default:
				return instr, fmt.Errorf("zc*: Q0/%03b, raw=%04x",
					funct3, raw)
			}

		case 0b101: // C.FSD
			instr.Rs2 = compressedRegisters[raw>>2&0b111]
			instr.Rs1 = compressedRegisters[raw>>7&0b111]
			instr.Imm = int32(raw&0b11100_00000000)>>7 |
				int32(raw&0b1100000)<<1
			instr.Op = Fsd

		case 0b110: // C.SW
			instr.Rs2 = compressedRegisters[raw>>2&0b111]
			instr.Rs1 = compressedRegisters[raw>>7&0b111]
			instr.Imm = int32(raw&0b1000000)>>4 |
				int32(raw&0b11100_00000000)>>7 |
				int32(raw&0b100000)<<1
			instr.Op = Sw

		case 0b111: // C.SD
			instr.Rs2 = compressedRegisters[raw>>2&0b111]
			instr.Rs1 = compressedRegisters[raw>>7&0b111]
			instr.Imm = int32(raw&0b11100_00000000)>>7 |
				int32(raw&0b1100000)<<1
			instr.Op = Sd

		default:
			return instr, fmt.Errorf("C: Q0/%03b, raw=%04x", funct3, raw)
		}

	case 0b01: // Quadrant 1
		switch funct3 {
		case 0b000: // C.ADDI / C.NOP
			instr.Rd = rds1
			instr.Rs1 = rds1
			instr.Imm = int32(raw&0b1111100)>>2 |
				int32(raw&0b10000_00000000)>>7
			if instr.Imm&0b100000 != 0 {
				// XXX change all sign extends to use this pattern
				instr.Imm |= ^int32(0b111111)
			}
			instr.Op = Addi

		case 0b001: // C.ADDIW
			instr.Rd = rds1
			instr.Rs1 = rds1
			instr.Imm = int32(raw&0b1111100)>>2 |
				int32(raw&0b10000_00000000)>>7
			if instr.Imm&0b100000 != 0 {
				instr.Imm |= ^int32(0b111111)

			}
			instr.Op = Addiw

		case 0b010: // C.LI
			instr.Rd = rds1
			instr.Rs1 = Zero
			instr.Imm = int32(raw&0b1111100)>>2 |
				int32(raw&0b10000_00000000)>>7
			if instr.Imm&0b100000 != 0 {
				instr.Imm |= ^int32(0b111111)
			}
			instr.Op = Addi

		case 0b011: // C.ADDI16SP / C.LUI
			instr.Rd = rds1
			if rds1 == 2 {
				// C.ADDI16SP
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
				// C.LUI
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
			case 0b00: // C.SRLI
				instr.Rd = compressedRegisters[raw>>7&0b111]
				instr.Rs1 = instr.Rd
				instr.Imm = int32(raw&0b1111100)>>2 |
					int32(raw&0b10000_00000000)>>7
				instr.Op = Srli

			case 0b01: // C.SRAI
				instr.Rd = compressedRegisters[raw>>7&0b111]
				instr.Rs1 = instr.Rd
				instr.Imm = int32(raw&0b1111100)>>2 |
					int32(raw&0b10000_00000000)>>7
				instr.Op = Srai

			case 0b10: // C.ANDI
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
				case 0b000: // C.SUB
					instr.Rd = compressedRegisters[raw>>7&0b111]
					instr.Rs1 = instr.Rd
					instr.Rs2 = compressedRegisters[raw>>2&0b111]
					instr.Op = Sub

				case 0b001: // C.XOR
					instr.Rd = compressedRegisters[raw>>7&0b111]
					instr.Rs1 = instr.Rd
					instr.Rs2 = compressedRegisters[raw>>2&0b111]
					instr.Op = Xor

				case 0b010: // C.OR
					instr.Rd = compressedRegisters[raw>>7&0b111]
					instr.Rs1 = instr.Rd
					instr.Rs2 = compressedRegisters[raw>>2&0b111]
					instr.Op = Or

				case 0b011: // C.AND
					instr.Rd = compressedRegisters[raw>>7&0b111]
					instr.Rs1 = instr.Rd
					instr.Rs2 = compressedRegisters[raw>>2&0b111]
					instr.Op = And

				case 0b100: // C.SUBW
					instr.Rd = compressedRegisters[raw>>7&0b111]
					instr.Rs1 = instr.Rd
					instr.Rs2 = compressedRegisters[raw>>2&0b111]
					instr.Op = Subw

				case 0b101: // C.ADDW
					instr.Rd = compressedRegisters[raw>>7&0b111]
					instr.Rs1 = instr.Rd
					instr.Rs2 = compressedRegisters[raw>>2&0b111]
					instr.Op = Addw

				case 0b110: // C.MUL
					// "Zc*" Extension for Code Size Reduction.
					instr.Rd = compressedRegisters[raw>>7&0b111]
					instr.Rs1 = instr.Rd
					instr.Rs2 = compressedRegisters[raw>>2&0b111]
					instr.Op = Mul

				case 0b111:
					// "Zc*" Extension for Code Size Reduction.
					//
					// At this point, we have resolved quadrant Q1 [1:0] and
					// bits [15:10] and [6:5].
					switch raw >> 2 & 0b111 {
					case 0b000: // C.ZEXT.B
						instr.Rd = compressedRegisters[raw>>7&0b111]
						instr.Rs1 = instr.Rd
						instr.Rs2 = Zero
						instr.Imm = 0xff
						instr.Op = Andi

					case 0b001: // C.SEXT.B
						instr.Rd = compressedRegisters[raw>>7&0b111]
						instr.Rs1 = instr.Rd
						instr.Rs2 = Zero
						instr.Op = SextB

					case 0b010: // C.ZEXT.H
						instr.Rd = compressedRegisters[raw>>7&0b111]
						instr.Rs1 = instr.Rd
						instr.Rs2 = Zero
						instr.Imm = 0xffff
						instr.Op = Andi

					case 0b011: // C.SEXT.H
						instr.Rd = compressedRegisters[raw>>7&0b111]
						instr.Rs1 = instr.Rd
						instr.Rs2 = Zero
						instr.Op = SextH

					case 0b100: // C.ZEXT.W
						instr.Rd = compressedRegisters[raw>>7&0b111]
						instr.Rs1 = instr.Rd
						instr.Rs2 = Zero
						instr.Op = AddUw

					case 0b101: // C.NOT
						instr.Rd = compressedRegisters[raw>>7&0b111]
						instr.Rs1 = instr.Rd
						instr.Rs2 = Zero
						instr.Imm = -1
						instr.Op = Xori

					default:
						return instr,
							fmt.Errorf("zcb: Q1/100/11/%03b, raw=%04x", f3, raw)
					}

				default:
					return instr, fmt.Errorf("C: Q1/100/11/%03b, raw=%04x",
						f3, raw)
				}

			default:
				return instr,
					fmt.Errorf("C: Q1/%03b/%02b, raw=%04x", funct3, funct2, raw)
			}

		case 0b101: // C.J
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

		case 0b110, 0b111: // C.BEQZ / C.BNEZ
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
				// C.BEQZ
				instr.Op = Beq
			} else {
				// C.BNEZ
				instr.Op = Bne
			}

		default:
			return instr,
				fmt.Errorf("C: Q1/%03b, raw=%04x", funct3, raw)
		}

	case 0b10: // Quadrant 2
		switch funct3 {
		case 0b000: // C.SLLI
			instr.Rd = rds1
			instr.Rs1 = rds1
			instr.Imm = int32(raw&0b1111100)>>2 |
				int32(raw&0b10000_00000000)>>7
			instr.Op = Slli

		case 0b001: // C.FLDSP
			instr.Imm = int32(raw&0b11100)<<4 |
				int32(raw&0b1100000)>>2 |
				int32(raw&0b10000_00000000)>>7
			instr.Rd = rds1
			instr.Rs1 = Sp
			instr.Op = Fld

		case 0b010: // C.LWSP
			instr.Imm = int32(raw&0b1110000)>>2 |
				int32(raw&0b10000_00000000)>>7 |
				int32(raw&0b1100)<<4
			instr.Rd = rds1
			instr.Rs1 = Sp
			instr.Op = Lw

		case 0b011: // C.LDSP
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
						// C.JR
						instr.Rs1 = rds1
						instr.Op = Jalr
					} else {
						// C.MV
						instr.Rd = rds1
						instr.Rs2 = rs2
						instr.Op = Add
					}
				} else {
					return instr, fmt.Errorf("C: Q2, raw=%04x", raw)
				}
			} else {
				if rds1 == 0 {
					if rs2 == 0 {
						// C.EBREAK
						instr.Op = Ebreak
					} else {
						return instr, fmt.Errorf("C: Q2, raw=%04x", raw)
					}
				} else {
					if rs2 == 0 {
						// C.JALR
						instr.Rd = Ra
						instr.Rs1 = rds1
						instr.Op = Jalr
					} else {
						// C.ADD
						instr.Rd = rds1
						instr.Rs1 = rds1
						instr.Rs2 = rs2
						instr.Op = Add
					}
				}
			}

		case 0b101: // C.FSDSP
			instr.Rs1 = Sp
			instr.Rs2 = rs2
			instr.Imm = int32(raw&0b11100_00000000)>>7 |
				int32(raw&0b11_10000000)>>1
			instr.Op = Fsd

		case 0b110: // C.SWSP
			instr.Rs1 = Sp
			instr.Rs2 = rs2
			instr.Imm = int32(raw&0b11110_00000000)>>7 |
				int32(raw&0b01_10000000)>>1
			instr.Op = Sw

		case 0b111: // C.SDSP
			instr.Rs1 = Sp
			instr.Rs2 = rs2
			instr.Imm = int32(raw&0b11100_00000000)>>7 |
				int32(raw&0b11_10000000)>>1
			instr.Op = Sd

		default:
			return instr, fmt.Errorf("C: Q2/%03b, raw=%04x", funct3, raw)
		}

	default:
		return instr, fmt.Errorf("C: Q%v, raw=%04x", raw&0b11, raw)
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
		case 0b000:
			instr.Op = Addi

		case 0b001:
			funct6 := raw >> 26
			switch funct6 {
			case 0b000000:
				instr.Op = Slli
			case 0b001010:
				instr.Op = Bseti
			case 0b010010:
				instr.Op = Bclri
			case 0b011000:
				switch raw >> 20 & 0b111111 {
				case 0b000000:
					instr.Op = Clz
				case 0b000001:
					instr.Op = Ctz
				case 0b000010:
					instr.Op = Cpop
				case 0b000100:
					instr.Op = SextB
				case 0b000101:
					instr.Op = SextH
				default:
					return instr, fmt.Errorf("%v/%03b/%06b/%06b: raw=%x",
						group, funct3, funct6, raw>>20&0b111111, raw)
				}
			case 0b011010:
				instr.Op = Binvi
			default:
				return instr, fmt.Errorf("%v/%03b/%06b: raw=%x",
					group, funct3, funct6, raw)
			}
			instr.Imm &= 0b111111

		case 0b010:
			instr.Op = Slti
		case 0b011:
			instr.Op = Sltiu
		case 0b100:
			instr.Op = Xori

		case 0b101:
			switch funct7 >> 1 {
			case 0b000000:
				instr.Op = Srli

			case 0b001010:
				instr.Op = OrcB

			case 0b010000:
				instr.Op = Srai

			case 0b010010:
				instr.Op = Bexti

			case 0b011000:
				// XXX for rori, the shmat is 6 bits but below masks 7
				// bits, which is ok as our lowest bit is zero. Check
				// the formats above if imm is 6 or 7 bits.
				instr.Op = Rori

			case 0b011010:
				switch raw >> 20 & 0b111111 {
				case 0b111000:
					instr.Op = Rev8

				default:
					return instr, fmt.Errorf("%v/%03b/%06b/%06b: raw=%x",
						group, funct3, funct7>>1, raw>>20&0b111111, raw)
				}

			default:
				return instr, fmt.Errorf("%v/%03b/%06b: raw=%x",
					group, funct3, funct7>>1, raw)
			}
			instr.Imm &= 0b1111111

		case 0b110:
			instr.Op = Ori
		case 0b111:
			instr.Op = Andi
		}

	case GroupOPIMM32:
		instr.typeI(raw)
		switch funct3 {
		case 0b000:
			instr.Op = Addiw
		case 0b001:
			funct6 := raw >> 26
			instr.Imm &= 0b111111
			switch funct6 {
			case 0b000000:
				instr.Op = Slliw
			case 0b000010:
				instr.Op = SlliUW
			case 0b011000:
				instr.Op = Clzw
			default:
				return instr, fmt.Errorf("%v/%03b/%06b, raw=%x",
					group, funct3, funct6, raw)
			}
		case 0b101:
			switch funct7 {
			case 0b0000000:
				instr.Op = Srliw
			case 0b0100000:
				instr.Op = Sraiw
				instr.Imm &= 0b111111
			case 0b0110000:
				instr.Op = Roriw
				instr.Imm &= 0b11111
			default:
				return instr, fmt.Errorf("%v/%03b/%07b, raw=%x",
					group, funct3, funct7, raw)
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
		case 0b0000000:
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
					fmt.Errorf("%v/%07b/%03b: raw=%08x",
						group, funct7, funct3, raw)
			}

			// The 'M' Extension (Multiply/Divide)
		case 0b0000001:
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

			// The 'B' Extension for Bit Manipulation.
		case 0b0000101:
			switch funct3 {
			case 0b100:
				instr.Op = Min

			case 0b101:
				instr.Op = Minu

			case 0b110:
				instr.Op = Max

			case 0b111:
				instr.Op = Maxu

			default:
				return instr,
					fmt.Errorf("%v: B-extension: funct3=%03b, raw=%08x",
						group, funct3, raw)
			}

			// Zicond Extension (Conditional Integer Operations).
		case 0b0000111:
			switch funct3 {
			case 0b101:
				instr.Op = CzeroEqz
			case 0b111:
				instr.Op = CzeroNez
			default:
				return instr,
					fmt.Errorf("zicond: funct3=%03b, raw=%08x", funct3, raw)
			}

			// Zbs (Single-Bit Instructions) extension.
		case 0b0010100:
			switch funct3 {
			case 0b001:
				instr.Op = Bset
			default:
				return instr,
					fmt.Errorf("%v zbs: funct3=%03b, raw=%08x",
						group, funct3, raw)
			}

		case 0b0100000:
			switch funct3 {
			case 0b000:
				instr.Op = Sub
			case 0b101:
				instr.Op = Sra
			case 0b110:
				instr.Op = Orn
			case 0b111:
				instr.Op = Andn
			default:
				return instr,
					fmt.Errorf("%v/%07b/%03b: raw=%08x",
						group, funct7, funct3, raw)
			}

			// Zbs Extension for Single-bit instructions.
		case 0b0100100:
			switch funct3 {
			case 0b101:
				instr.Op = Bext

			default:
				return instr,
					fmt.Errorf("%v/%07b/%03b: raw=%08x",
						group, funct7, funct3, raw)
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
			return instr, fmt.Errorf("%v/%07b: raw=%08x", group, funct7, raw)
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

		case 0b0000100:
			switch funct3 {
			case 0b000:
				instr.Op = AddUw
			case 0b100: // ZextH: X(rsdc) = EXTZ(X(rsdc)[15..0])
				instr.Imm = 0xffff
				instr.Op = Andi
			default:
				return instr, fmt.Errorf("group %v: funct7=%v, raw=%x",
					group, funct7, raw)
			}

		case 16:
			switch funct3 {
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
		switch funct3 {
		case 0b000:
			instr.Op = Fence
		case 0b001:
			instr.Op = FenceI

		case 0b010: // CBO
			funct12 := raw >> 20
			switch funct12 {
			case 0b000000000000:
				instr.Op = CboInval
			case 0b000000000001:
				instr.Op = CboClean
			case 0b000000000010:
				instr.Op = CboFlush
			case 0b000000000100:
				instr.Op = CboZero
			default:
				return instr, fmt.Errorf("%v/%03b/%012b: raw=%x",
					group, funct3, funct12, raw)
			}

		case 0b110: // ORI
			funct5 := raw >> 20 & 0b11111
			switch funct5 {
			case 0b00000:
				instr.Op = PrefetchI
			case 0b00001:
				instr.Op = PrefetchR
			case 0b00011:
				instr.Op = PrefetchW
			default:
				return instr, fmt.Errorf("%v/%03b/%05b: raw=%x",
					group, funct3, funct5, raw)
			}

		default:
			return instr, fmt.Errorf("%v/%03b: raw=%x",
				group, funct3, raw)
		}

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
			}

		case 0b00001:
			switch funct3 {
			case 2:
				instr.Op = AmoswapW
			case 3:
				instr.Op = AmoswapD
			}

		case 0b00010:
			switch funct3 {
			case 2:
				instr.Op = LrW
			case 3:
				instr.Op = LrD
			}

		case 0b00011:
			switch funct3 {
			case 2:
				instr.Op = ScW
			case 3:
				instr.Op = ScD
			}

		case 0b00100:
			switch funct3 {
			case 2:
				instr.Op = AmoxorW
			case 3:
				instr.Op = AmoxorD
			}

		case 0b01000:
			switch funct3 {
			case 2:
				instr.Op = AmoorW
			case 3:
				instr.Op = AmoorD
			}

		case 0b01100:
			switch funct3 {
			case 2:
				instr.Op = AmoandW
			case 3:
				instr.Op = AmoandD
			}

		case 0b10000:
			switch funct3 {
			case 2:
				instr.Op = AmominW
			case 3:
				instr.Op = AmominD
			}

		case 0b10100:
			switch funct3 {
			case 2:
				instr.Op = AmomaxW
			case 3:
				instr.Op = AmomaxD
			}

		case 0b11000:
			switch funct3 {
			case 2:
				instr.Op = AmominuW
			case 3:
				instr.Op = AmominuD
			}

		case 0b11100:
			switch funct3 {
			case 2:
				instr.Op = AmomaxuW
			case 3:
				instr.Op = AmomaxuD
			}

		default:
			return instr, fmt.Errorf("AMO/%05b/%03b: raw=%08x",
				funct5, funct3, raw)
		}

	case GroupLOADFP:
		instr.Imm = int32(raw) >> 20

		switch funct3 {
		// Vector loads.
		case 0b000:
			lumop := raw >> 20 & 0b11111
			switch lumop {
			case 0b00000:
				instr.Op = Vle8V
			case 0b01000:
				instr.Op = Vl1re8V
			case 0b10000: // vle8ff.v - lumop is checked at VPU
				instr.Op = Vle8V
			case 0b01011:
				instr.Op = VlmV
			default:
				return instr, fmt.Errorf("%v/%03b/%05b, raw=%08x",
					group, funct3, lumop, raw)
			}

		case 0b010:
			instr.Op = Flw

		case 0b011:
			instr.Op = Fld

		case 0b101:
			instr.Op = Vle16V

		case 0b110:
			instr.Op = Vle32V

		case 0b111:
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

		case 0b0010000:
			switch funct3 {
			case 0b000:
				instr.Op = FsgnjS
			case 0b001:
				instr.Op = FsgnjnS
			case 0b010:
				instr.Op = FsgnjxS
			}

		case 0b0010100:
			switch funct3 {
			case 0b000:
				instr.Op = FminS
			case 0b001:
				instr.Op = FmaxS
			}

		case 0b0010101:
			switch funct3 {
			case 0b000:
				instr.Op = FminD
			case 0b001:
				instr.Op = FmaxD
			}

		case 0b0101100:
			instr.Op = FsqrtS

		case 0b0101101:
			instr.Op = FsqrtD
		case 0b1010000:
			switch funct3 {
			case 0b000:
				instr.Op = FleS
			case 0b001:
				instr.Op = FltS
			case 0b010:
				instr.Op = FeqS
			}
		case 0b1010001:
			switch funct3 {
			case 0b000:
				instr.Op = FleD
			case 0b001:
				instr.Op = FltD
			case 0b010:
				instr.Op = FeqD
			}

		case 0b1100000:
			funct5 := raw >> 20 & 0b11111
			switch funct5 {
			case 0b00000:
				instr.Op = FcvtWS
			case 0b00001:
				instr.Op = FcvtWUS
			case 0b00010:
				instr.Op = FcvtLS
			case 0b00011:
				instr.Op = FcvtLUS
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
			case 0b00011:
				instr.Op = FcvtLUD
			}

		case 0b1101000:
			funct5 := raw >> 20 & 0b11111
			switch funct5 {
			case 0b00000:
				instr.Op = FcvtSW
			case 0b00001:
				instr.Op = FcvtSWU
			case 0b0010:
				instr.Op = FcvtSL
			case 0b00011:
				instr.Op = FcvtSLU
			}

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
			}

		case 0b1110000:
			switch funct3 {
			case 0b000:
				instr.Op = FmvXW
			case 0b001:
				instr.Op = FclassS
			}

		case 0b1110001:
			switch funct3 {
			case 0b000:
				instr.Op = FmvXD
			case 0b001:
				instr.Op = FclassD
			}

		case 0b1111000:
			instr.Op = FmvWX
		case 0b1111001:
			instr.Op = FmvDX

		case 0b0010001:
			switch funct3 {
			case 0b000:
				instr.Op = FsgnjD
			case 0b001:
				instr.Op = FsgnjnD
			case 0b010:
				instr.Op = FsgnjxD
			}

		case 0b0100000:
			instr.Op = FcvtSD

		case 0b0100001:
			instr.Op = FcvtDS

		default:
			return instr, fmt.Errorf("OP-FP/%07b/%03b: raw=%08x",
				funct7, funct3, raw)
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
		case 0b000: // OPIVV
			switch funct7 >> 1 {
			case 0b000000:
				instr.Imm = int32(funct7 & 0b1) // Store bit 0: VM
				instr.Op = VaddVv

			case 0b000110:
				instr.Imm = int32(funct7 & 0b1) // Store bit 0: VM
				instr.Op = VmaxuVv

			case 0b001011:
				instr.Imm = int32(funct7 & 0b1) // Store bit 0: VM
				instr.Op = VxorVv

			case 0b001010:
				instr.Imm = int32(funct7 & 0b1) // Store bit 0: VM
				instr.Op = VorVv

			case 0b001100:
				instr.Imm = int32(funct7 & 0b1) // Store bit 0: VM
				instr.Op = VrgatherVv

			case 0b010111:
				if funct7&0b1 == 1 {
					instr.Op = VmvVV
				} else {
					instr.Op = VmergeVvm
				}

			case 0b011001:
				instr.Imm = int32(funct7 & 0b1) // Store bit 0: VM
				instr.Op = VmsneVv

			default:
				return instr, fmt.Errorf("%v/%03b/%06b: raw=%x",
					group, funct3, funct7>>1, raw)
			}

		case 0b010: // OPMVV
			switch funct7 >> 1 {
			case 0b000000:
				instr.Imm = int32(funct7 & 0b1) // Store bit 0: VM
				instr.Op = VredsumVs

			case 0b000011:
				instr.Imm = int32(funct7 & 0b1) // Store bit 0: VM
				instr.Op = VredxorVs

			case 0b010000:
				instr.Imm = int32(funct7 & 0b1) // Store bit 0: VM
				instr.Op = VfirstM

			case 0b010100:
				instr.Imm = int32(funct7 & 0b1) // Store bit 0: VM
				instr.Op = VidV

			case 0b010010:
				instr.Imm = int32(funct7 & 0b1) // Store bit 0: VM
				switch instr.Rs1 {
				// Table 22. VXUNARY0 encoding space:
				// 00010 vzext.vf8
				// 00011 vsext.vf8
				// 00100 vzext.vf4
				// 00101 vsext.vf4
				// 00110 vzext.vf2
				// 00111 vsext.vf2
				case 0b00011:
					instr.Op = VsextVf8
				case 0b00101:
					instr.Op = VsextVf4

				default:
					return instr, fmt.Errorf("%v/%03b/%06b, vs1=%05b, raw=%x",
						group, funct3, funct7>>1, int(instr.Rs1), raw)
				}

			case 0b011010:
				instr.Imm = int32(funct7 & 0b1) // Store bit 0: VM
				instr.Op = VmorMM

			default:
				return instr, fmt.Errorf("%v/%03b/%06b: raw=%x",
					group, funct3, funct7>>1, raw)
			}

		case 0b011: // OPIVI
			switch funct7 >> 1 {
			case 0b000000:
				vm := funct7 & 1
				// Imm is sign-extented and shifted one bit left.
				imm := uint32(int32(raw<<12) >> 26)
				// We store vm bit to bit 0.
				if vm != 0 {
					imm |= 1
				} else {
					imm &^= 1
				}
				instr.Imm = int32(imm)
				instr.Op = VaddVi

			case 0b000011:
				vm := funct7 & 1
				// Imm is sign-extented and shifted one bit left.
				imm := uint32(int32(raw<<12) >> 26)
				// We store vm bit to bit 0.
				if vm != 0 {
					imm |= 1
				} else {
					imm &^= 1
				}
				instr.Imm = int32(imm)
				instr.Op = VrsubVi

			case 0b001110:
				vm := funct7 & 1
				// Imm is sign-extented and shifted one bit left.
				imm := uint32(raw >> 14 & 0b111110)
				// We store vm bit to bit 0.
				if vm != 0 {
					imm |= 1
				} else {
					imm &^= 1
				}
				instr.Imm = int32(imm)
				instr.Op = VslideupVi

			case 0b010111:
				instr.Imm = int32(raw<<12) >> 27
				instr.Op = VmvVI

			case 0b100101:
				vm := funct7 & 1
				imm := uint32(instr.Rs1)
				// We store vm bit to bit 31.
				if vm != 0 {
					imm |= 1 << 31
				}
				instr.Imm = int32(imm)
				instr.Op = VsllVi

			case 0b101000:
				vm := funct7 & 1
				imm := uint32(instr.Rs1)
				// We store vm bit to bit 31.
				if vm != 0 {
					imm |= 1 << 31
				}
				instr.Imm = int32(imm)
				instr.Op = VsrlVi

			case 0b101001:
				vm := funct7 & 1
				imm := uint32(instr.Rs1)
				// We store vm bit to bit 31.
				if vm != 0 {
					imm |= 1 << 31
				}
				instr.Imm = int32(imm)
				instr.Op = VsraVi

			case 0b011000:
				vm := funct7 & 1
				// Imm is sign-extented and shifted one bit left.
				imm := uint32(int32(raw<<12) >> 26)
				// We store vm bit to bit 0.
				if vm != 0 {
					imm |= 1
				} else {
					imm &^= 1
				}
				instr.Imm = int32(imm)
				instr.Op = VmseqVi

			default:
				return instr, fmt.Errorf("%v/%03b/%06b: raw=%x",
					group, funct3, funct7>>1, raw)
			}

		case 0b100: // OPIVX
			switch funct7 >> 1 {
			case 0b000000:
				instr.Imm = int32(funct7 & 0b1) // Store bit 0: VM
				instr.Op = VaddVx

			case 0b001001:
				instr.Imm = int32(funct7 & 0b1) // Store bit 0: VM
				instr.Op = VandVx

			case 0b010111:
				instr.Imm = int32(funct7 & 0b1) // Store bit 0: VM
				instr.Op = VmvVX

			default:
				return instr, fmt.Errorf("%v/%03b/%06b: raw=%x",
					group, funct3, funct7>>1, raw)
			}

		case 0b110: // OPMVX
			switch funct7 >> 1 {
			case 0b010000:
				instr.Imm = int32(funct7 & 0b1) // Store bit 0: VM
				instr.Op = VmvSX

			case 0b100101:
				instr.Imm = int32(funct7 & 0b1) // Store bit 0: VM
				instr.Op = VmulVx

			default:
				return instr, fmt.Errorf("%v/%03b/%06b: raw=%x",
					group, funct3, funct7>>1, raw)
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
			return instr, fmt.Errorf("%v/%03b: raw=%x", group, funct3, raw)
		}

	case GroupMSUB:
		funct2 := raw >> 25 & 0b11
		switch funct2 {
		case 0b00:
			instr.Imm = int32(raw >> 27 & 0b11111)
			instr.Op = FmsubS
		case 0b01:
			instr.Imm = int32(raw >> 27 & 0b11111)
			instr.Op = FmsubD
		default:
			return instr, fmt.Errorf("%v: funct2=%02b, raw=%08x",
				group, funct2, raw)
		}

	case GroupNMSUB:
		funct2 := raw >> 25 & 0b11
		switch funct2 {
		case 0b00:
			instr.Imm = int32(raw >> 27 & 0b11111)
			instr.Op = FnmsubS
		case 0b01:
			instr.Imm = int32(raw >> 27 & 0b11111)
			instr.Op = FnmsubD
		default:
			return instr, fmt.Errorf("%v: funct2=%02b, raw=%08x",
				group, funct2, raw)
		}

	case GroupNMADD:
		funct2 := raw >> 25 & 0b11
		switch funct2 {
		case 0b01:
			instr.Imm = int32(raw >> 27 & 0b11111)
			instr.Op = FnmaddD
		default:
			return instr, fmt.Errorf("%v: funct2=%02b, raw=%08x",
				group, funct2, raw)
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
