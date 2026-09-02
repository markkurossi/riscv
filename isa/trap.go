//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package isa

import (
	"fmt"
	"os"
	"runtime/debug"
)

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

// Exception cause codes.
const (
	CauseInstAddrMisaligned = iota
	CauseInstAccessFault
	CauseIllegalInstr
	CauseBreakpoint
	CauseLoadAddrMisaligned
	CauseLoadAccessFault
	CauseStoreAddrMisaligned
	CauseStoreAccessFault
	CauseEcallU
	CauseEcallS
	CauseEcallVS
	CauseEcallM
	CauseInstPageFault
	CauseLoadPageFault
	_
	CauseStorePageFault
	CauseDoubleTrap
	_
	CauseSoftwareCheck
	CauseHardwareError
	CauseInstGuestPageFault
	CauseLoadGuestPageFault
	CauseVirtualInst
	CauseStoreGuestPageFault
)

var exceptionCauses = map[uint64]string{
	CauseInstAddrMisaligned:  "Instruction address misaligned",
	CauseInstAccessFault:     "Instruction access fault",
	CauseIllegalInstr:        "Illegal instruction",
	CauseBreakpoint:          "Breakpoint",
	CauseLoadAddrMisaligned:  "Load address misaligned",
	CauseLoadAccessFault:     "Load access fault",
	CauseStoreAddrMisaligned: "Store/AMO address misaligned",
	CauseStoreAccessFault:    "Store/AMO access fault",
	CauseEcallU:              "Environment call from U-mode",
	CauseEcallS:              "Environment call from S-mode",
	CauseEcallVS:             "Environment call from VS-mode",
	CauseEcallM:              "Environment call from M-mode",
	CauseInstPageFault:       "Instruction page fault",
	CauseLoadPageFault:       "Load page fault",
	CauseStorePageFault:      "Store/AMO page fault",
	CauseDoubleTrap:          "Double trap",
	CauseSoftwareCheck:       "Software check",
	CauseHardwareError:       "Hardware error",
	CauseInstGuestPageFault:  "Instruction guest-page fault",
	CauseLoadGuestPageFault:  "Load guest-page fault",
	CauseVirtualInst:         "Virtual instruction",
	CauseStoreGuestPageFault: "Store/AMO guest-page fault",
}

// Interrupt cause codes.
const (
	CauseReserved0Intr = iota
	CauseSupervisorSoftwareInter
	CauseReserved2Intr
	CauseMachineSoftwareInter
	CauseReserved4Intr
	CauseSupervisorTimerInter
	CauseReserved6Intr
	CauseMachineTimerInter
	CauseReserved8Intr
	CauseSupervisorExternalInter
	CauseReserved10Intr
	CauseMachineExternalInter
	CauseReserved12Int
	CauseCounterOverflowInter
	CauseReserved14Intr
	CauseReserved15Intr
)

var interruptCauses = map[uint64]string{
	CauseReserved0Intr:           "Reserved 0 interrupt",
	CauseSupervisorSoftwareInter: "Supervisor software interrupt",
	CauseReserved2Intr:           "Reserved 2 interrupt",
	CauseMachineSoftwareInter:    "Machine software interrupt",
	CauseReserved4Intr:           "Reserved 4 interrupt",
	CauseSupervisorTimerInter:    "Supervisor timer interrupt",
	CauseReserved6Intr:           "Reserved 6 interrupt",
	CauseMachineTimerInter:       "Machine timer interrupt",
	CauseReserved8Intr:           "Reserved 8 interrupt",
	CauseSupervisorExternalInter: "Supervisor external interrupt",
	CauseReserved10Intr:          "Reserved 10 interrupt",
	CauseMachineExternalInter:    "Machine external interrupt",
	CauseReserved12Int:           "Reserved 12 interrupt",
	CauseCounterOverflowInter:    "Counter-overflow interrupt",
	CauseReserved14Intr:          "Reserved 14 interrupt",
	CauseReserved15Intr:          "Reserved 15 interrupt",
}

// Trap encapsulates runtime exception and interrupt information.
type Trap struct {
	PC    uint64
	Tval  uint64
	Cause uint64
	Err   error
}

// NewTrap creates a new trap.
func NewTrap(pc, cause, tval uint64, err error) *Trap {
	if false {
		fmt.Printf("Trap: pc=%x, cause=%v, tval=%x, err=%v\n",
			pc, cause, tval, err)
		if false {
			debug.PrintStack()
			os.Exit(1)
		}
	}
	return &Trap{
		PC:    pc,
		Tval:  tval,
		Cause: cause,
		Err:   err,
	}
}

func (trap *Trap) Error() string {
	var name string
	var ok bool
	if trap.Cause>>63 != 0 {
		cause := trap.Cause & ^(uint64(1) << 63)
		name, ok = interruptCauses[cause]
		if !ok {
			name = fmt.Sprintf("Interrupt %d", cause)
		}
	} else {
		name, ok = exceptionCauses[trap.Cause]
		if !ok {
			name = fmt.Sprintf("Exception %d", trap.Cause)
		}
	}
	return fmt.Sprintf("%s: pc=%x, tval=%x", name, trap.PC, trap.Tval)
}

func (trap *Trap) Unwrap() error {
	return trap.Err
}
