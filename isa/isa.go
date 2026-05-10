//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

// Package isa implements the RISC-V Instruction Set Architecture
// (ISA).
package isa

import (
	"fmt"
)

// Group defines instruction groups.
type Group uint8

// RV32/64G instruction groups.
const (
	GroupLOAD    Group = 0x03
	GroupLOADFP  Group = 0x07
	GroupCustom0 Group = 0x0b
	GroupMISCMEM Group = 0x0f
	GroupOPIMM   Group = 0x13
	GroupAUIPC   Group = 0x17
	GroupOPIMM32 Group = 0x1b
	GroupSTORE   Group = 0x23
	GroupSTOREFP Group = 0x27
	GroupCustom1 Group = 0x2b
	GroupAMO     Group = 0x2f
	GroupOP      Group = 0x33
	GroupLUI     Group = 0x37
	GroupOP32    Group = 0x3b
	GroupMADD    Group = 0x43
	GroupMSUB    Group = 0x47
	GroupNMSUB   Group = 0x4b
	GroupNMADD   Group = 0x4f
	GroupOPFP    Group = 0x53
	GroupOPV     Group = 0x57
	GroupCustom2 Group = 0x5b
	GroupBRANCH  Group = 0x63
	GroupJALR    Group = 0x67
	GroupJAL     Group = 0x6f
	GroupSYSTEM  Group = 0x73
	GroupOPVE    Group = 0x77
	GroupCustom3 Group = 0x7b
)

var groups = map[Group]string{
	GroupLOAD:    "LOAD",
	GroupLOADFP:  "LOAD-FP",
	GroupCustom0: "custom-0",
	GroupMISCMEM: "MISC-MEM",
	GroupOPIMM:   "OP-IMM",
	GroupAUIPC:   "AUIPC",
	GroupOPIMM32: "OP-IMM-32",
	GroupSTORE:   "STORE",
	GroupSTOREFP: "STORE-FP",
	GroupCustom1: "custom-1",
	GroupAMO:     "AMO",
	GroupOP:      "OP",
	GroupLUI:     "LUI",
	GroupOP32:    "OP-32",
	GroupMADD:    "MADD",
	GroupMSUB:    "MSUB",
	GroupNMSUB:   "NMSUB",
	GroupNMADD:   "NMADD",
	GroupOPFP:    "OP-FP",
	GroupOPV:     "OP-V",
	GroupCustom2: "custom-2",
	GroupBRANCH:  "BRANCH",
	GroupJALR:    "JALR",
	GroupJAL:     "JAL",
	GroupSYSTEM:  "SYSTEM",
	GroupOPVE:    "OP-VE",
	GroupCustom3: "custom-3",
}

func (g Group) String() string {
	name, ok := groups[g]
	if ok {
		return name
	}
	return fmt.Sprintf("{Group %x}", int(g))
}

// Op defines instruction opcodes.
type Op int

// Known RISC-V opcodes.
const (
	// Invalid / unknown.
	Invalid Op = iota

	// Integer arithmetic (RV64I).

	// Register-register.
	Add
	Sub
	Sll
	Slt
	Sltu
	Xor
	Srl
	Sra
	Or
	And

	// Immediate.
	Addi
	Slti
	Sltiu
	Xori
	Ori
	Andi
	Slli
	Srli
	Srai

	// 32-bit (RV64).
	Addw
	Subw
	Sllw
	Srlw
	Sraw

	Addiw
	Slliw
	Srliw
	Sraiw

	// Multiplication / Division (M extension).
	Mul
	Mulh
	Mulhsu
	Mulhu
	Div
	Divu
	Rem
	Remu

	// 32-bit variants.
	Mulw
	Divw
	Divuw
	Remw
	Remuw

	// Memory operations.

	// Loads.
	Lb
	Lh
	Lw
	Ld
	Lbu
	Lhu
	Lwu

	// Stores.
	Sb
	Sh
	Sw
	Sd

	// Control flow.
	Beq
	Bne
	Blt
	Bge
	Bltu
	Bgeu

	Jal
	Jalr

	// Upper immediates.

	Lui
	Auipc

	// System / CSR.

	Ecall
	Ebreak

	Csrrw
	Csrrs
	Csrrc
	Csrrwi
	Csrrsi
	Csrrci

	Mret
	Sret
	Wfi

	// Memory ordering.

	Fence
	FenceI

	// Atomic (A extension).

	LrW
	ScW
	AmoaddW
	AmoswapW
	AmoxorW
	AmoandW
	AmoorW
	AmominW
	AmomaxW
	AmominuW
	AmomaxuW

	LrD
	ScD
	AmoaddD
	AmoswapD
	AmoxorD
	AmoandD
	AmoorD
	AmominD
	AmomaxD
	AmominuD
	AmomaxuD

	// Floating point loads/stores.

	Flw
	Fld
	Fsw
	Fsd

	// Floating point arithmetic (F/D).

	FaddS
	FaddD
	FsubS
	FsubD
	FmulS
	FmulD
	FdivS
	FdivD

	FsqrtS
	FsqrtD

	// Sign injection.
	FsgnjS
	FsgnjnS
	FsgnjxS
	FsgnjD
	FsgnjnD
	FsgnjxD

	// Min/max.
	FminS
	FmaxS
	FminD
	FmaxD

	// Comparisons.
	FeqS
	FltS
	FleS
	FeqD
	FltD
	FleD

	// Conversions.

	FcvtWS
	FcvtWUS
	FcvtLS
	FcvtLUS

	FcvtWD
	FcvtWUD
	FcvtLD
	FcvtLUD

	FcvtSW
	FcvtSWU
	FcvtSL
	FcvtSLU

	FcvtDW
	FcvtDWU
	FcvtDL
	FcvtDLU

	// Move / classify.

	FmvXW
	FmvXD
	FmvWX
	FmvDX

	FclassS
	FclassD

	// Floating-point Fused Multiply-Add
	FmaddS
	FmaddD

	// Zba & Zbb Bit-Manipulation Extensions.
	AddUw
	Sh1addUw
	Sh2addUw
	Sh3addUw
	Rolw
	Rorw
)

// OpInfo defines opcode information.
type OpInfo struct {
	Name  string
	Usage string
	Desc  string
}

var Operands = map[Op]OpInfo{
	Invalid: OpInfo{
		Name: "invalid",
	},
	Add: OpInfo{
		Name: "add",
		Desc: "Add",
	},
	Sub: OpInfo{
		Name: "sub",
	},
	Sll: OpInfo{
		Name: "sll",
	},
	Slt: OpInfo{
		Name: "slt",
	},
	Sltu: OpInfo{
		Name: "sltu",
	},
	Xor: OpInfo{
		Name: "xor",
	},
	Srl: OpInfo{
		Name: "srl",
	},
	Sra: OpInfo{
		Name: "sra",
	},
	Or: OpInfo{
		Name: "or",
		Desc: "OR",
	},
	And: OpInfo{
		Name: "and",
	},
	Addi: OpInfo{
		Name: "addi",
		Desc: "Add Immediate",
	},
	Slti: OpInfo{
		Name: "slti",
	},
	Sltiu: OpInfo{
		Name: "sltiu",
	},
	Xori: OpInfo{
		Name: "xori",
	},
	Ori: OpInfo{
		Name: "ori",
	},
	Andi: OpInfo{
		Name: "andi",
		Desc: "AND Immediate",
	},
	Slli: OpInfo{
		Name: "slli",
		Desc: "Shift Left Logical Immediate",
	},
	Srli: OpInfo{
		Name: "srli",
	},
	Srai: OpInfo{
		Name: "srai",
	},
	Addw: OpInfo{
		Name: "addw",
	},
	Subw: OpInfo{
		Name: "subw",
	},
	Sllw: OpInfo{
		Name: "sllw",
	},
	Srlw: OpInfo{
		Name: "srlw",
	},
	Sraw: OpInfo{
		Name: "sraw",
	},
	Addiw: OpInfo{
		Name: "addiw",
		Desc: "Add Word Immediate",
	},
	Slliw: OpInfo{
		Name: "slliw",
	},
	Srliw: OpInfo{
		Name: "srliw",
	},
	Sraiw: OpInfo{
		Name: "sraiw",
	},
	Mul: OpInfo{
		Name: "mul",
	},
	Mulh: OpInfo{
		Name: "mulh",
	},
	Mulhsu: OpInfo{
		Name: "mulhsu",
	},
	Mulhu: OpInfo{
		Name: "mulhu",
	},
	Div: OpInfo{
		Name: "div",
	},
	Divu: OpInfo{
		Name: "divu",
	},
	Rem: OpInfo{
		Name: "rem",
	},
	Remu: OpInfo{
		Name: "remu",
	},
	Mulw: OpInfo{
		Name: "mulw",
	},
	Divw: OpInfo{
		Name: "divw",
	},
	Divuw: OpInfo{
		Name: "divuw",
	},
	Remw: OpInfo{
		Name: "remw",
	},
	Remuw: OpInfo{
		Name: "remuw",
	},
	Lb: OpInfo{
		Name: "lb",
	},
	Lh: OpInfo{
		Name: "lh",
	},
	Lw: OpInfo{
		Name: "lw",
		Desc: "Load Word",
	},
	Ld: OpInfo{
		Name: "ld",
		Desc: "Load Doubleword",
	},
	Lbu: OpInfo{
		Name: "lbu",
		Desc: "Load Byte, Unsigned",
	},
	Lhu: OpInfo{
		Name: "lhu",
	},
	Lwu: OpInfo{
		Name: "lwu",
	},
	Sb: OpInfo{
		Name: "sb",
	},
	Sh: OpInfo{
		Name: "sh",
	},
	Sw: OpInfo{
		Name: "sw",
	},
	Sd: OpInfo{
		Name: "sd",
		Desc: "Store Doubleword",
	},
	Beq: OpInfo{
		Name: "beq",
		Desc: "Branch if Equal",
	},
	Bne: OpInfo{
		Name: "bne",
		Desc: "Branch if Not Equal",
	},
	Blt: OpInfo{
		Name: "blt",
		Desc: "Branch if Less Than",
	},
	Bge: OpInfo{
		Name: "bge",
	},
	Bltu: OpInfo{
		Name: "bltu",
	},
	Bgeu: OpInfo{
		Name: "bgeu",
		Desc: "Branch if Greater Than or Equal",
	},
	Jal: OpInfo{
		Name: "jal",
		Desc: "Jump and Link",
	},
	Jalr: OpInfo{
		Name: "jalr",
		Desc: "Jump and Link Register",
	},
	Lui: OpInfo{
		Name: "lui",
	},
	Auipc: OpInfo{
		Name: "auipc",
		Desc: "Add Upper Immediate to PC",
	},
	Ecall: OpInfo{
		Name: "ecall",
	},
	Ebreak: OpInfo{
		Name: "ebreak",
	},
	Csrrw: OpInfo{
		Name: "csrrw",
	},
	Csrrs: OpInfo{
		Name: "csrrs",
	},
	Csrrc: OpInfo{
		Name: "csrrc",
	},
	Csrrwi: OpInfo{
		Name: "csrrwi",
	},
	Csrrsi: OpInfo{
		Name: "csrrsi",
	},
	Csrrci: OpInfo{
		Name: "csrrci",
	},
	Mret: OpInfo{
		Name: "mret",
	},
	Sret: OpInfo{
		Name: "sret",
	},
	Wfi: OpInfo{
		Name: "wfi",
	},
	Fence: OpInfo{
		Name: "fence",
	},
	FenceI: OpInfo{
		Name: "fence.i",
	},
	LrW: OpInfo{
		Name: "lr.w",
	},
	ScW: OpInfo{
		Name: "sc.w",
	},
	AmoaddW: OpInfo{
		Name: "amoadd.w",
	},
	AmoswapW: OpInfo{
		Name: "amoswap.w",
	},
	AmoxorW: OpInfo{
		Name: "amoxor.w",
	},
	AmoandW: OpInfo{
		Name: "amoand.w",
	},
	AmoorW: OpInfo{
		Name: "amoor.w",
	},
	AmominW: OpInfo{
		Name: "amomin.w",
	},
	AmomaxW: OpInfo{
		Name: "amomax.w",
	},
	AmominuW: OpInfo{
		Name: "amominu.w",
	},
	AmomaxuW: OpInfo{
		Name: "amomaxu.w",
	},
	LrD: OpInfo{
		Name: "lr.d",
	},
	ScD: OpInfo{
		Name: "sc.d",
	},
	AmoaddD: OpInfo{
		Name: "amoadd.d",
	},
	AmoswapD: OpInfo{
		Name: "amoswap.d",
	},
	AmoxorD: OpInfo{
		Name: "amoxor.d",
	},
	AmoandD: OpInfo{
		Name: "amoand.d",
	},
	AmoorD: OpInfo{
		Name: "amoor.d",
	},
	AmominD: OpInfo{
		Name: "amomin.d",
	},
	AmomaxD: OpInfo{
		Name: "amomax.d",
	},
	AmominuD: OpInfo{
		Name: "amominu.d",
	},
	AmomaxuD: OpInfo{
		Name: "amomaxu.d",
	},
	Flw: OpInfo{
		Name: "flw",
	},
	Fld: OpInfo{
		Name: "fld",
	},
	Fsw: OpInfo{
		Name: "fsw",
	},
	Fsd: OpInfo{
		Name: "fsd",
	},
	FaddS: OpInfo{
		Name: "fadd.s",
	},
	FaddD: OpInfo{
		Name: "fadd.d",
	},
	FsubS: OpInfo{
		Name: "fsub.s",
	},
	FsubD: OpInfo{
		Name: "fsub.d",
	},
	FmulS: OpInfo{
		Name: "fmul.s",
	},
	FmulD: OpInfo{
		Name: "fmul.d",
	},
	FdivS: OpInfo{
		Name: "fdiv.s",
	},
	FdivD: OpInfo{
		Name: "fdiv.d",
	},
	FsqrtS: OpInfo{
		Name: "fsqrt.s",
	},
	FsqrtD: OpInfo{
		Name: "fsqrt.d",
	},
	FsgnjS: OpInfo{
		Name: "fsgnj.s",
	},
	FsgnjnS: OpInfo{
		Name: "fsgnjn.s",
	},
	FsgnjxS: OpInfo{
		Name: "fsgnjx.s",
	},
	FsgnjD: OpInfo{
		Name: "fsgnj.d",
	},
	FsgnjnD: OpInfo{
		Name: "fsgnjn.d",
	},
	FsgnjxD: OpInfo{
		Name: "fsgnjx.d",
	},
	FminS: OpInfo{
		Name: "fmin.s",
	},
	FmaxS: OpInfo{
		Name: "fmax.s",
	},
	FminD: OpInfo{
		Name: "fmin.d",
	},
	FmaxD: OpInfo{
		Name: "fmax.d",
	},
	FeqS: OpInfo{
		Name: "feq.s",
	},
	FltS: OpInfo{
		Name: "flt.s",
	},
	FleS: OpInfo{
		Name: "fle.s",
	},
	FeqD: OpInfo{
		Name: "feq.d",
	},
	FltD: OpInfo{
		Name: "flt.d",
	},
	FleD: OpInfo{
		Name: "fle.d",
	},
	FcvtWS: OpInfo{
		Name: "fcvt.w.s",
	},
	FcvtWUS: OpInfo{
		Name: "fcvt.w.us",
	},
	FcvtLS: OpInfo{
		Name: "fcvt.l.s",
	},
	FcvtLUS: OpInfo{
		Name: "fcvt.lu.s",
	},
	FcvtWD: OpInfo{
		Name: "fcvt.w.d",
	},
	FcvtWUD: OpInfo{
		Name: "fcvt.wu.d",
	},
	FcvtLD: OpInfo{
		Name: "fcvt.l.d",
	},
	FcvtLUD: OpInfo{
		Name: "fcvt.lu.d",
	},
	FcvtSW: OpInfo{
		Name: "fcvt.s.w",
	},
	FcvtSWU: OpInfo{
		Name: "fcvt.s.wu",
	},
	FcvtSL: OpInfo{
		Name: "fcvt.s.l",
	},
	FcvtSLU: OpInfo{
		Name: "fcvt.s.lu",
	},
	FcvtDW: OpInfo{
		Name: "fcvt.d.w",
	},
	FcvtDWU: OpInfo{
		Name: "fcvt.d.wu",
	},
	FcvtDL: OpInfo{
		Name: "fcvt.d.l",
	},
	FcvtDLU: OpInfo{
		Name: "fcvt.d.lu",
	},
	FmvXW: OpInfo{
		Name: "fmv.x.w",
	},
	FmvXD: OpInfo{
		Name: "fmv.x.d",
	},
	FmvWX: OpInfo{
		Name: "fmv.w.x",
	},
	FmvDX: OpInfo{
		Name: "fmv.d.x",
	},
	FclassS: OpInfo{
		Name: "fclass.s",
	},
	FclassD: OpInfo{
		Name: "fclass.d",
	},
	FmaddS: OpInfo{
		Name: "fmadd.s",
	},
	FmaddD: OpInfo{
		Name: "fmadd.d",
	},
	AddUw: OpInfo{
		Name: "add.uw",
	},
	Sh1addUw: OpInfo{
		Name: "sh1add.uw",
	},
	Sh2addUw: OpInfo{
		Name: "sh2add.uw",
	},
	Sh3addUw: OpInfo{
		Name: "sh3add.uw",
	},
	Rolw: OpInfo{
		Name: "rolw",
	},
	Rorw: OpInfo{
		Name: "rorw",
	},
}

const (
	maxOpNameLen = 9
)

func (op Op) String() string {
	info, ok := Operands[op]
	if ok {
		return info.Name
	}
	return fmt.Sprintf("{Op %d}", op)
}

// Register defines RISC-V registers.
type Register uint8

const (
	Zero Register = iota
	Ra
	Sp
	Gp
	Tp
	T0
	T1
	T2
	S0
	S1
	A0
	A1
	A2
	A3
	A4
	A5
	A6
	A7
	S2
	S3
	S4
	S5
	S6
	S7
	S8
	S9
	S10
	S11
	T3
	T4
	T5
	T6
)

var registers = [32]string{
	"zero", // x0
	"ra",   // x1
	"sp",   // x2
	"gp",   // x3
	"tp",   // x4
	"t0",   // x5
	"t1",   // x6
	"t2",   // x7
	"s0",   // x8
	"s1",   // x9
	"a0",   // x10
	"a1",   // x11
	"a2",   // x12
	"a3",   // x13
	"a4",   // x14
	"a5",   // x15
	"a6",   // x16
	"a7",   // x17
	"s2",   // x18
	"s3",   // x19
	"s4",   // x20
	"s5",   // x21
	"s6",   // x22
	"s7",   // x23
	"s8",   // x24
	"s9",   // x25
	"s10",  // x26
	"s11",  // x27
	"t3",   // x28
	"t4",   // x29
	"t5",   // x30
	"t6",   // x31
}

func (r Register) String() string {
	if int(r) < len(registers) {
		return registers[r]
	}
	return fmt.Sprintf("x%d", r)
}

// Instr defines RISC-V instructions.
type Instr struct {
	Op  Op
	Rd  Register
	Rs1 Register
	Rs2 Register
	Imm int32
}

func pad(op Op) string {
	name := op.String()
	for len(name) < maxOpNameLen {
		name += " "
	}
	return name
}

func (instr Instr) String() string {
	if instr.Op != Invalid {
		switch instr.Op {
		case Add, And, Div, Divu, Divw, Mul, Mulhu, Mulw, Or, Rem, Remw,
			Slt, Sll, Sltu, Srl, Sub, Xor:
			// GroupOP, GroupOP32
			return fmt.Sprintf("%v %v,%v,%v",
				pad(instr.Op), instr.Rd, instr.Rs1, instr.Rs2)

		case Addi, Addiw, Andi, Slli, Slliw, Slti, Sltiu, Srai, Sraiw,
			Srli, Srliw, Ori, Xori:
			// GroupOPIMM, GroupOPIMM32
			return fmt.Sprintf("%v %v,%v,%d",
				pad(instr.Op), instr.Rd, instr.Rs1, instr.Imm)

		case Auipc: // GroupAUIPC
			return fmt.Sprintf("%v %v,0x%x",
				pad(instr.Op), instr.Rd, uint32(instr.Imm)>>12)

		case Beq, Bge, Bgeu, Blt, Bltu, Bne: // GroupBRANCH
			return fmt.Sprintf("%v %v,%v,%d",
				pad(instr.Op), instr.Rs1, instr.Rs2, instr.Imm)

		case Ecall: // GroupSYSTEM
			return pad(Ecall)

		case Jal: // GroupJAL
			return fmt.Sprintf("%v %d", pad(instr.Op), instr.Imm)

		case Jalr: // GroupJALR
			return fmt.Sprintf("%v %v,%d(%v)",
				pad(instr.Op), instr.Rd, instr.Imm, instr.Rs1)

		case Lb, Lbu, Ld, Lhu, Lw, Lwu: // GroupLOAD
			return fmt.Sprintf("%v %v,%d(%v)",
				pad(instr.Op), instr.Rd, instr.Imm, instr.Rs1)

		case Fld, Flw:
			return fmt.Sprintf("%v F%v,%d(%v)",
				pad(instr.Op), int(instr.Rd), instr.Imm, instr.Rs1)

		case Lui: // GroupLUI
			return fmt.Sprintf("%v %v,0x%x",
				pad(instr.Op), instr.Rd, uint32(instr.Imm)>>12)

		case Sb, Sd, Sh, Sw: // GroupSTORE
			return fmt.Sprintf("%v %v,%d(%v)",
				pad(instr.Op), instr.Rs2, instr.Imm, instr.Rs1)

			// GroupAMO
		case LrD, LrW:
			return fmt.Sprintf("%v %v(%v)",
				pad(instr.Op), instr.Rd, instr.Rs1)
		case AmoaddD, AmoaddW, AmoandD, AmoandW, AmoorD, AmoorW,
			AmoswapD, AmoswapW, ScD, ScW:
			return fmt.Sprintf("%v %v,%v(%v)",
				pad(instr.Op), instr.Rd, instr.Rs2, instr.Rs1)

			// GroupMISCMEM
		case Fence:
			return pad(instr.Op)
		}
	}

	return fmt.Sprintf("Instr: Op=%v", instr.Op)
}

func (instr *Instr) typeI(raw uint32) {
	instr.Imm = int32(raw) >> 20
}

func (instr *Instr) typeS(raw uint32) {
	sraw := int32(raw)

	instr.Imm = (sraw>>20)&^0b11111 | ((sraw >> 7) & 0b11111)
}

func (instr *Instr) typeB(raw uint32) {
	sraw := int32(raw)

	if false {
		fmt.Printf("raw   : %b\n", sraw)
		fmt.Printf(" 12   : %13b\n", (sraw>>19)&^0b01111_11111111)
		fmt.Printf(" 11   : %13b\n", (sraw&0b00000_10000000)<<4)
		fmt.Printf(" 10:5 : %13b\n", (sraw>>20)&0b00111_11100000)
		fmt.Printf(" 4:1  : %13b\n", (sraw>>7)&0b00000_00011110)
	}

	instr.Imm = (sraw>>19)&^0b01111_11111111 |
		(sraw&0b00000_10000000)<<4 |
		(sraw>>20)&0b00111_11100000 |
		(sraw>>7)&0b00000_00011110
}

func (instr *Instr) typeU(raw uint32) {
	instr.Imm = int32(raw &^ 0b1111_11111111)
}

func (instr *Instr) typeJ(raw uint32) {
	sraw := int32(raw)

	if false {
		fmt.Printf("raw   : %b\n", sraw)
		fmt.Printf(" 20   : %21b\n", (sraw>>11)&^0b1111_11111111_11111111)
		fmt.Printf(" 19:12: %21b\n", (sraw & 0b1111_11110000_00000000))
		fmt.Printf(" 11   : %21b\n", (sraw>>9)&0b1000_00000000)
		fmt.Printf(" 10:1 : %21b\n", (sraw>>20)&0b111_11111110)
	}

	instr.Imm = (sraw>>11)&^0b1111_11111111_11111111 |
		(sraw & 0b1111_11110000_00000000) |
		(sraw>>9)&0b1000_00000000 |
		(sraw>>20)&0b111_11111110
}

func (instr *Instr) op() string {
	return instr.Op.String()
}
