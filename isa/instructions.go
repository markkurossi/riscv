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

type Group uint8

const (
	GroupLOAD    Group = 0x03
	GroupLOADFP  Group = 0x07
	GroupMISCMEM Group = 0x0f
	GroupOPIMM   Group = 0x13
	GroupAUIPC   Group = 0x17
	GroupOPIMM32 Group = 0x1b
	GroupSTORE   Group = 0x23
	GroupSTOREFP Group = 0x27
	GroupAMO     Group = 0x2f
	GroupOP      Group = 0x33
	GroupLUI     Group = 0x37
	GroupOP32    Group = 0x3b
	GroupOPFP    Group = 0x53
	GroupBRANCH  Group = 0x63
	GroupJALR    Group = 0x67
	GroupJAL     Group = 0x6f
	GroupSYSTEM  Group = 0x73
)

var groups = map[Group]string{
	GroupLOAD:    "LOAD",
	GroupLOADFP:  "LOAD-FP",
	GroupMISCMEM: "MISC-MEM",
	GroupOPIMM:   "OP-IMM",
	GroupAUIPC:   "AUIPC",
	GroupOPIMM32: "OP-IMM-32",
	GroupSTORE:   "STORE",
	GroupSTOREFP: "STORE-FP",
	GroupAMO:     "AMO",
	GroupOP:      "OP",
	GroupLUI:     "LUI",
	GroupOP32:    "OP-32",
	GroupOPFP:    "OP-FP",
	GroupBRANCH:  "BRANCH",
	GroupJALR:    "JALR",
	GroupJAL:     "JAL",
	GroupSYSTEM:  "SYSTEM",
}

func (g Group) String() string {
	name, ok := groups[g]
	if ok {
		return name
	}
	return fmt.Sprintf("{Group %d}", g)
}

type Op int

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
)

var instrs = map[Op]string{
	Add:      "add",
	Sub:      "sub",
	Sll:      "sll",
	Slt:      "slt",
	Sltu:     "sltu",
	Xor:      "xor",
	Srl:      "srl",
	Sra:      "sra",
	Or:       "or",
	And:      "and",
	Addi:     "addi",
	Slti:     "slti",
	Sltiu:    "sltiu",
	Xori:     "xori",
	Ori:      "ori",
	Andi:     "andi",
	Slli:     "slli",
	Srli:     "srli",
	Srai:     "srai",
	Addw:     "addw",
	Subw:     "subw",
	Sllw:     "sllw",
	Srlw:     "srlw",
	Sraw:     "sraw",
	Addiw:    "addiw",
	Slliw:    "slliw",
	Srliw:    "srliw",
	Sraiw:    "sraiw",
	Mul:      "mul",
	Mulh:     "mulh",
	Mulhsu:   "mulhsu",
	Mulhu:    "mulhu",
	Div:      "div",
	Divu:     "divu",
	Rem:      "rem",
	Remu:     "remu",
	Mulw:     "mulw",
	Divw:     "divw",
	Divuw:    "divuw",
	Remw:     "remw",
	Remuw:    "remuw",
	Lb:       "lb",
	Lh:       "lh",
	Lw:       "lw",
	Ld:       "ld",
	Lbu:      "lbu",
	Lhu:      "lhu",
	Lwu:      "lwu",
	Sb:       "sb",
	Sh:       "sh",
	Sw:       "sw",
	Sd:       "sd",
	Beq:      "beq",
	Bne:      "bne",
	Blt:      "blt",
	Bge:      "bge",
	Bltu:     "bltu",
	Bgeu:     "bgeu",
	Jal:      "jal",
	Jalr:     "jalr",
	Lui:      "lui",
	Auipc:    "auipc",
	Ecall:    "ecall",
	Ebreak:   "ebreak",
	Csrrw:    "csrrw",
	Csrrs:    "csrrs",
	Csrrc:    "csrrc",
	Csrrwi:   "csrrwi",
	Csrrsi:   "csrrsi",
	Csrrci:   "csrrci",
	Mret:     "mret",
	Sret:     "sret",
	Wfi:      "wfi",
	Fence:    "fence",
	FenceI:   "fence.i",
	LrW:      "lr.w",
	ScW:      "sc.w",
	AmoaddW:  "amoadd.w",
	AmoswapW: "amoswap.w",
	AmoxorW:  "amoxor.w",
	AmoandW:  "amoand.w",
	AmoorW:   "amoor.w",
	AmominW:  "amomin.w",
	AmomaxW:  "amomax.w",
	AmominuW: "amominu.w",
	AmomaxuW: "amomaxu.w",
	LrD:      "lr.d",
	ScD:      "sc.d",
	AmoaddD:  "amoadd.d",
	AmoswapD: "amoswap.d",
	AmoxorD:  "amoxor.d",
	AmoandD:  "amoand.d",
	AmoorD:   "amoor.d",
	AmominD:  "amomin.d",
	AmomaxD:  "amomax.d",
	AmominuD: "amominu.d",
	AmomaxuD: "amomaxu.d",
	Flw:      "flw",
	Fld:      "fld",
	Fsw:      "fsw",
	Fsd:      "fsd",
	FaddS:    "fadd.s",
	FaddD:    "fadd.d",
	FsubS:    "fsub.s",
	FsubD:    "fsub.d",
	FmulS:    "fmul.s",
	FmulD:    "fmul.d",
	FdivS:    "fdiv.s",
	FdivD:    "fdiv.d",
	FsqrtS:   "fsqrt.s",
	FsqrtD:   "fsqrt.d",
	FsgnjS:   "fsgnj.s",
	FsgnjnS:  "fsgnjn.s",
	FsgnjxS:  "fsgnjx.s",
	FsgnjD:   "fsgnj.d",
	FsgnjnD:  "fsgnjn.d",
	FsgnjxD:  "fsgnjx.d",
	FminS:    "fmin.s",
	FmaxS:    "fmax.s",
	FminD:    "fmin.d",
	FmaxD:    "fmax.d",
	FeqS:     "feq.s",
	FltS:     "flt.s",
	FleS:     "fle.s",
	FeqD:     "feq.d",
	FltD:     "flt.d",
	FleD:     "fle.d",
	FcvtWS:   "fcvt.w.s",
	FcvtWUS:  "fcvt.w.us",
	FcvtLS:   "fcvt.l.s",
	FcvtLUS:  "fcvt.lu.s",
	FcvtWD:   "fcvt.w.d",
	FcvtWUD:  "fcvt.wu.d",
	FcvtLD:   "fcvt.l.d",
	FcvtLUD:  "fcvt.lu.d",
	FcvtSW:   "fcvt.s.w",
	FcvtSWU:  "fcvt.s.wu",
	FcvtSL:   "fcvt.s.l",
	FcvtSLU:  "fcvt.s.lu",
	FcvtDW:   "fcvt.d.w",
	FcvtDWU:  "fcvt.d.wu",
	FcvtDL:   "fcvt.d.l",
	FcvtDLU:  "fcvt.d.lu",
	FmvXW:    "fmv.x.w",
	FmvXD:    "fmv.x.d",
	FmvWX:    "fmv.w.x",
	FmvDX:    "fmv.d.x",
	FclassS:  "fclass.s",
	FclassD:  "fclass.d",
}

func (op Op) String() string {
	name, ok := instrs[op]
	if ok {
		return name
	}
	return fmt.Sprintf("{Op %d}", op)
}
