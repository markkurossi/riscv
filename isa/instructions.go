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

type OpInfo struct {
	Name  string
	Usage string
	Desc  string
}

var instrs = map[Op]OpInfo{
	Add: OpInfo{
		Name: "add",
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
	},
	And: OpInfo{
		Name: "and",
	},
	Addi: OpInfo{
		Name: "addi",
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
	},
	Slli: OpInfo{
		Name: "slli",
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
	},
	Ld: OpInfo{
		Name: "ld",
	},
	Lbu: OpInfo{
		Name: "lbu",
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
	},
	Beq: OpInfo{
		Name: "beq",
	},
	Bne: OpInfo{
		Name: "bne",
	},
	Blt: OpInfo{
		Name: "blt",
	},
	Bge: OpInfo{
		Name: "bge",
	},
	Bltu: OpInfo{
		Name: "bltu",
	},
	Bgeu: OpInfo{
		Name: "bgeu",
	},
	Jal: OpInfo{
		Name: "jal",
	},
	Jalr: OpInfo{
		Name: "jalr",
	},
	Lui: OpInfo{
		Name: "lui",
	},
	Auipc: OpInfo{
		Name: "auipc",
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
}

func (op Op) String() string {
	info, ok := instrs[op]
	if ok {
		return info.Name
	}
	return fmt.Sprintf("{Op %d}", op)
}
