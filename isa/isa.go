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
	"strings"
)

const (
	MisaA uint64 = 1 << iota // Atomic extension
	MisaB                    // B extension
	MisaC                    // Compressed extension
	MisaD                    // Double-precision floating-point extension
	MisaE                    // RV32E/64E base ISA
	MisaF                    // Single-precision floating-point extension
	MisaG                    // Reserved
	MisaH                    // Hypervisor extension
	MisaI                    // RV32I/64I base ISA
	MisaJ                    // Reserved
	MisaK                    // Reserved
	MisaL                    // Reserved
	MisaM                    // Integer Multiply/Divide extension
	MisaN                    // Tentatively reserved for User-Level Interrupts extension
	MisaO                    // Reserved
	MisaP                    // Tentatively reserved for Packed-SIMD extension
	MisaQ                    // Quad-precision floating-point extension
	MisaR                    // Reserved
	MisaS                    // Supervisor mode implemented
	MisaT                    // Reserved
	MisaU                    // User mode implemented
	MisaV                    // Vector extension
	MisaW                    // Reserved
	MisaX                    // Non-standard extensions present
	MisaY                    // Reserved
	MisaZ                    // Reserved

	MisaMXL uint64 = 2 << 62
)

type PrivilegeMode uint8

const (
	ModeU PrivilegeMode = iota
	ModeS
	ModeH
	ModeM
)

var modes = map[PrivilegeMode]string{
	ModeU: "U",
	ModeS: "S",
	ModeH: "H",
	ModeM: "M",
}

func (m PrivilegeMode) String() string {
	name, ok := modes[m]
	if ok {
		return name
	}
	return fmt.Sprintf("{PrivilegeMode %d}", int(m))
}

// XXX Thise are interrupt cause code.
//
// Bit  Name   Meaning
// ─────────────────────────────────────────
//
//		0   USIP   User Software Interrupt (mip only, pending)
//		1   SSIP   Supervisor Software Interrupt
//		2   —      reserved
//		3   MSIP   Machine Software Interrupt
//		4   UTIP   User Timer Interrupt
//		5   STIP   Supervisor Timer Interrupt
//		6   —      reserved
//		7   MTIP   Machine Timer Interrupt
//		8   UEIP   User External Interrupt
//		9   SEIP   Supervisor External Interrupt
//	   10   —      reserved
//	   11   MEIP   Machine External Interrupt
//	   12   —      reserved (SGEIP in hypervisor ext)
//	   13+  —      platform-defined / reserved
const (
	IntUSIP = 1 << iota
	IntSSIP
	_
	IntMSIP
	IntUTIP
	IntSTIP
	_
	IntMTIP
	IntUEIP
	IntSEIP
	_
	IntMEIP
)

func IntString(v uint64) string {
	var result []string
	if v&IntMEIP != 0 {
		result = append(result, "MEIP")
	}
	if v&IntSEIP != 0 {
		result = append(result, "SEIP")
	}
	if v&IntUEIP != 0 {
		result = append(result, "UEIP")
	}
	if v&IntMTIP != 0 {
		result = append(result, "MTIP")
	}

	if v&IntSTIP != 0 {
		result = append(result, "STIP")
	}
	if v&IntUTIP != 0 {
		result = append(result, "UTIP")
	}
	if v&IntMSIP != 0 {
		result = append(result, "MSIP")
	}
	if v&IntSSIP != 0 {
		result = append(result, "SSIP")
	}
	if v&IntUSIP != 0 {
		result = append(result, "USIP")
	}
	return strings.Join(result, ",")
}

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

type VType int32

func (vt VType) VLMUL() float32 {
	switch vt & 0b111 {
	case 0b000:
		return 1.0
	case 0b001:
		return 2.0
	case 0b010:
		return 4.0
	case 0b011:
		return 8.0
	case 0b111:
		return 0.5
	case 0b110:
		return 0.25
	case 0b101:
		return 0.125
	default:
		return 1.0
	}
}

func (vt VType) VSEW() uint8 {
	return uint8(8 << ((vt >> 3) & 0b111))
}

func (vt VType) VTA() bool {
	return vt&(1<<6) != 0
}

func (vt VType) VMA() bool {
	return vt&(1<<7) != 0
}

func (vt VType) String() string {
	result := fmt.Sprintf("e%v", vt.VSEW())

	var lmul string
	switch vt & 0b111 {
	case 0b000:
		lmul = "m1"
	case 0b001:
		lmul = "m2"
	case 0b010:
		lmul = "m4"
	case 0b011:
		lmul = "m8"
	case 0b111:
		lmul = "mf2"
	case 0b110:
		lmul = "mf4"
	case 0b101:
		lmul = "mf8"
	default:
		lmul = "reserved"
	}
	result += "," + lmul

	if vt&(1<<6) != 0 {
		result += ",ta"
	}
	if vt&(1<<7) != 0 {
		result += ",ma"
	}

	return result
}

// Op defines instruction opcodes. The standard RISC-V Base Integer
// ISA (RV32I/RV64I) has about 40–50 instructions. The M (Multiply), A
// (Atomic), F/D (Floating Point), and C (Compressed) extensions,
// bring us around 120–150 unique opcodes. The current maximum value
// Rorw is 157 so we have 98 opcodes left.
type Op uint8

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
	SfenceVMA

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
	FcvtSD
	FcvtDS

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

	// Vector extension.

	Vsetvli
	Vsetivli
	Vsetvl
	VmvVX
	VmvVI
	Vle8V
	Vle16V
	Vle32V
	Vle64V
	Vse8V
	Vse16V
	Vse32V
	Vse64V

	// Zicond Extension (Conditional Integer Operations).
	CzeroEqz
	CzeroNez

	// Zba & Zbb Bit-Manipulation Extensions.
	AddUw
	Maxu
	Sh1add
	Sh2add
	Sh3add
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
		Desc: "CSR Read and Write",
	},
	Csrrs: OpInfo{
		Name: "csrrs",
		Desc: "CSR Read and Set",
	},
	Csrrc: OpInfo{
		Name: "csrrc",
	},
	Csrrwi: OpInfo{
		Name: "csrrwi",
		Desc: "CSR Read and Write Immediate",
	},
	Csrrsi: OpInfo{
		Name: "csrrsi",
		Desc: "CSR Read and Set Immediate",
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
	SfenceVMA: OpInfo{
		Name: "sfence.vma",
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
	FcvtSD: OpInfo{
		Name: "fcvt.s.d",
	},
	FcvtDS: OpInfo{
		Name: "fcvt.d.s",
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

	Vsetvli: OpInfo{
		Name: "vsetvli",
	},
	Vsetivli: OpInfo{
		Name: "vsetivli",
	},
	Vsetvl: OpInfo{
		Name: "vsetvl",
	},
	VmvVX: OpInfo{
		Name: "vmv.v.x",
	},
	VmvVI: OpInfo{
		Name: "vmv.v.i",
	},
	Vle8V: OpInfo{
		Name: "vle8.v",
	},
	Vle16V: OpInfo{
		Name: "vle16.v",
	},
	Vle32V: OpInfo{
		Name: "vle32.v",
	},
	Vle64V: OpInfo{
		Name: "vle64.v",
	},
	Vse8V: OpInfo{
		Name: "vse8.v",
	},
	Vse16V: OpInfo{
		Name: "vse16.v",
	},
	Vse32V: OpInfo{
		Name: "vse32.v",
	},
	Vse64V: OpInfo{
		Name: "vse64.v",
	},

	CzeroEqz: OpInfo{
		Name: "czero.eqz",
	},
	CzeroNez: OpInfo{
		Name: "czero.nez",
	},

	AddUw: OpInfo{
		Name: "add.uw",
	},
	Maxu: OpInfo{
		Name: "maxu",
	},
	Sh1add: OpInfo{
		Name: "sh1add",
	},
	Sh2add: OpInfo{
		Name: "sh2add",
	},
	Sh3add: OpInfo{
		Name: "sh3add",
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
	maxOpNameLen = 10
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
			Slt, Sll, Sltu, Srl, Sub, Subw, Xor, AddUw, Maxu,
			CzeroEqz, CzeroNez:
			// GroupOP, GroupOP32
			return fmt.Sprintf("%v %v,%v,%v",
				pad(instr.Op), instr.Rd, instr.Rs1, instr.Rs2)

		case Addi, Addiw, Addw, Andi, Slli, Slliw, Slti, Sltiu, Srai, Sraiw,
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

			// GroupSYSTEM
		case Ebreak, Ecall, Sret, Mret, Wfi:
			return pad(instr.Op)
		case SfenceVMA:
			return fmt.Sprintf("%v %v,%v", pad(instr.Op), instr.Rs1, instr.Rs2)

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

		case Fsw:
			return fmt.Sprintf("%v F%v,%d(%v)",
				pad(instr.Op), int(instr.Rs2), instr.Imm, instr.Rs1)

		case FcvtDS:
			return fmt.Sprintf("%v F%v,F%v",
				pad(instr.Op), int(instr.Rd), int(instr.Rs1))

		case FmvXD:
			return fmt.Sprintf("%v %v,F%v",
				pad(instr.Op), instr.Rd, int(instr.Rs1))

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

			// CSR mappings.
		case Csrrs, Csrrc, Csrrw:
			return fmt.Sprintf("%v %v,%x,%v",
				pad(instr.Op), instr.Rd, instr.Imm, instr.Rs1)
		case Csrrwi, Csrrsi, Csrrci:
			return fmt.Sprintf("%v %v,%x,%d",
				pad(instr.Op), instr.Rd, instr.Imm, uint32(instr.Rs1))

			// GroupOPV

		case Vsetvli:
			return fmt.Sprintf("%v %v,%v,%v",
				pad(instr.Op), instr.Rd, instr.Rs1, VType(instr.Imm))
		case Vsetivli:
			return fmt.Sprintf("%v %v,%v,%v",
				pad(instr.Op), instr.Rd, uint32(instr.Rs1), VType(instr.Imm))
		case Vsetvl:
			return fmt.Sprintf("%v %v,%v,%v",
				pad(instr.Op), instr.Rd, instr.Rs1, VType(instr.Rs2))

		case VmvVX:
			result := fmt.Sprintf("%v v%v,%v",
				pad(instr.Op), int(instr.Rd), instr.Rs1)
			if instr.Imm == 0 {
				result += fmt.Sprintf(",v%d.t", int(instr.Rs2))
			}
			return result

		case VmvVI:
			return fmt.Sprintf("%v v%v,%v",
				pad(instr.Op), int(instr.Rd), instr.Imm)

			// Vector loads and stores.
		case Vle8V, Vle16V, Vle32V, Vle64V, Vse8V, Vse16V, Vse32V, Vse64V:
			return instr.vsString()
		}
	}

	return fmt.Sprintf("Instr: Op=%v", instr.Op)
}

func (instr Instr) vsString() string {
	var size, op string

	switch instr.Op {
	case Vle8V:
		size = "8"
		op = "l"
	case Vle16V:
		size = "16"
		op = "l"
	case Vle32V:
		size = "32"
		op = "l"
	case Vle64V:
		size = "64"
		op = "l"
	case Vse8V:
		size = "8"
		op = "s"
	case Vse16V:
		size = "16"
		op = "s"
	case Vse32V:
		size = "32"
		op = "s"
	case Vse64V:
		size = "64"
		op = "s"
	default:
		panic(instr)
	}

	var seg string
	nf := instr.Imm >> 4
	if nf > 0 {
		seg = fmt.Sprintf("seg%v", nf+1)
	}

	var idx string
	var idxReg string
	mop := instr.Imm >> 1
	switch mop {
	case 0b011:
		idx = "ux"
		size = "i" + size
		idxReg = fmt.Sprintf(",%v", instr.Rs2)
	case 0b111:
		idx = "ox"
		size = "i" + size
		idxReg = fmt.Sprintf(",%v", instr.Rs2)
	}

	opname := fmt.Sprintf("v%v%v%ve%v.v", op, idx, seg, size)
	for len(opname) < maxOpNameLen {
		opname += " "
	}

	return fmt.Sprintf("%v v%v,(%v)%v",
		opname, int(instr.Rd), instr.Rs1, idxReg)
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
