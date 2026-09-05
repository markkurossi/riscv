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

// Processor extension flags.
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
	MisaN                    // Reserved for User-Level Interrupts extension
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

// PrivilegeMode defines the CPU privilege modes.
type PrivilegeMode uint8

// Privilege modes.
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

// Mstatus implements the machine status register.
type Mstatus uint64

// WPRI = Reserved Writes Preserve, Reads Ignore
const (
	MsSIE   = 1
	MsMIE   = 3
	MsSPIE  = 5
	MsUBE   = 6
	MsMPIE  = 7
	MsSPP   = 8
	MsVS    = 9
	MsMPP   = 11
	MsFS    = 13
	MsXS    = 15
	MsMPRV  = 17
	MsSUM   = 18
	MsMXR   = 19
	MsTVM   = 20
	MsTW    = 21
	MsTSR   = 22
	MsSPELP = 23
	MsSDT   = 24
	MsUXL   = 32
	MsSXL   = 34
	MsSBE   = 36
	MsMBE   = 37
	MsGVA   = 38
	MsMPV   = 39
	MsMPELP = 41
	MsMDT   = 42
	MsSD    = 63
)

const (
	// SstatusMask defines the Sstatus bits that are stored in the
	// Mstatus.
	SstatusMask = Mstatus(0) |
		Mstatus(1)<<MsSIE |
		Mstatus(1)<<MsSPIE |
		Mstatus(1)<<MsUBE |
		Mstatus(1)<<MsSPP |
		Mstatus(3)<<MsVS |
		Mstatus(3)<<MsFS |
		Mstatus(3)<<MsXS |
		Mstatus(1)<<MsSUM |
		Mstatus(1)<<MsMXR |
		Mstatus(1)<<MsSPELP |
		Mstatus(1)<<MsSDT |
		Mstatus(3)<<MsUXL |
		Mstatus(1)<<MsSD

	MConstMask = Mstatus(0) |
		Mstatus(3)<<MsUXL
)

// SIE returns the global supervisor interrupt enable flag.
func (m Mstatus) SIE() bool {
	return m&(1<<MsSIE) != 0
}

// SetSIE sets the global supervisor interrupt enable flag.
func (m *Mstatus) SetSIE(v bool) {
	if v {
		*m |= 1 << MsSIE
	} else {
		*m &^= 1 << MsSIE
	}
}

// MIE returns the global machine interrupt enable flag.
func (m Mstatus) MIE() bool {
	return m&(1<<MsMIE) != 0
}

// SetMIE sets the global machine interrupt enable flag.
func (m *Mstatus) SetMIE(v bool) {
	if v {
		*m |= 1 << MsMIE
	} else {
		*m &^= 1 << MsMIE
	}
}

// SPIE returns the saved global supervisor interrupt enable flag.
func (m Mstatus) SPIE() bool {
	return m&(1<<MsSPIE) != 0
}

// SetSPIE sets the saved global supervisor interrupt enable flag.
func (m *Mstatus) SetSPIE(v bool) {
	if v {
		*m |= 1 << MsSPIE
	} else {
		*m &^= 1 << MsSPIE
	}
}

// MPIE returns the saved global machine interrupt enable flag.
func (m Mstatus) MPIE() bool {
	return m&(1<<MsMPIE) != 0
}

// SetMPIE sets the saved global machine interrupt enable flag.
func (m *Mstatus) SetMPIE(v bool) {
	if v {
		*m |= 1 << MsMPIE
	} else {
		*m &^= 1 << MsMPIE
	}
}

// SPP returns the saved supervisor privilege mode.
func (m Mstatus) SPP() PrivilegeMode {
	return PrivilegeMode(m >> MsSPP & 0b1)
}

// SetSPP sets the saved supervisor privilege mode.
func (m *Mstatus) SetSPP(mode PrivilegeMode) {
	if mode > ModeS {
		panic("SetSPP: invalid mode")
	}
	*m &^= 1 << MsSPP
	*m |= Mstatus(mode&0b1) << MsSPP
}

// TSR returns the TSR (Trap Supervisor Return) flag.
func (m Mstatus) TSR() bool {
	return m&(1<<MsTSR) != 0
}

// RegStatus defines the register status. This is used floating point
// and vector extensions.
type RegStatus uint8

// Register statuses.
const (
	RegOff = iota
	RegInitial
	RegClean
	RegDirty
)

var regStatuses = map[RegStatus]string{
	RegOff:     "off",
	RegInitial: "initial",
	RegClean:   "clean",
	RegDirty:   "dirty",
}

func (s RegStatus) String() string {
	name, ok := regStatuses[s]
	if ok {
		return name
	}
	return fmt.Sprintf("{RegStatus %d}", s)
}

// VS returns the vector extension state.
func (m Mstatus) VS() RegStatus {
	return RegStatus(m >> MsVS & 0b11)
}

// SetVS sets the vector extension state.
func (m *Mstatus) SetVS(s RegStatus) {
	*m &^= 0b11 << MsVS
	*m |= Mstatus(s&0b11) << MsVS
}

// MPP returns the saved machine privilege mode.
func (m Mstatus) MPP() PrivilegeMode {
	return PrivilegeMode(m >> MsMPP & 0b11)
}

// SetMPP sets the saved machine privilege mode.
func (m *Mstatus) SetMPP(mode PrivilegeMode) {
	*m &^= 0b11 << MsMPP
	*m |= Mstatus(mode&0b11) << MsMPP
}

// FS returns the floating point extension state.
func (m Mstatus) FS() RegStatus {
	return RegStatus(m >> MsFS & 0b11)
}

// SetFS sets the floating point extension state.
func (m *Mstatus) SetFS(s RegStatus) {
	*m &^= 0b11 << MsFS
	*m |= Mstatus(s&0b11) << MsFS
}

// SUM returns the permit Supervisor User Memory access flag.
func (m Mstatus) SUM() bool {
	return m&(1<<MsSUM) != 0
}

// SetSUM sets the permit Supervisor User Memory access flag.
func (m *Mstatus) SetSUM(v bool) {
	if v {
		*m |= 1 << MsSUM
	} else {
		*m &^= 1 << MsSUM
	}
}

// MXR returns the Make eXecutable Readable flag.
func (m Mstatus) MXR() bool {
	return m&(1<<MsMXR) != 0
}

// SetMXR sets the Make eXecutable Readable flag.
func (m *Mstatus) SetMXR(v bool) {
	if v {
		*m |= 1 << MsMXR
	} else {
		*m &^= 1 << MsMXR
	}
}

// TVM returns the Trap Virtual Memory flag.
func (m Mstatus) TVM() bool {
	return m&(1<<MsTVM) != 0
}

// SetTVM sets the Trap Virtual Memory flag.
func (m *Mstatus) SetTVM(v bool) {
	if v {
		*m |= 1 << MsTVM
	} else {
		*m &^= 1 << MsTVM
	}
}

// SD returns the combined dirty status flag.
func (m Mstatus) SD() bool {
	return m&(1<<MsSD) != 0
}

// SetSD sets the combined dirty status flag.
func (m *Mstatus) SetSD(v bool) {
	if v {
		*m |= 1 << MsSD
	} else {
		*m &^= 1 << MsSD
	}
}

// VType implements the vector type register.
type VType int32

// VLMUL returns the group multiplier (LMUL).
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

// VSEW returns the selected element width (SEW).
func (vt VType) VSEW() uint8 {
	return uint8(8 << ((vt >> 3) & 0b111))
}

// VTA returns the vector tail agnostic (VTA) flag.
func (vt VType) VTA() bool {
	return vt&(1<<6) != 0
}

// VMA returns the vector mask agnostic (VMA) flag.
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
