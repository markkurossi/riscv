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
	"strings"
)

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

// Interrupt causes. For interrupts, these are 1<<cause.
//
//	Bit  Name   Meaning
//	─────────────────────────────────────────
//
//	 0   USIP   User Software Interrupt (mip only, pending)
//	 1   SSIP   Supervisor Software Interrupt
//	 2   —      reserved
//	 3   MSIP   Machine Software Interrupt
//	 4   UTIP   User Timer Interrupt
//	 5   STIP   Supervisor Timer Interrupt
//	 6   —      reserved
//	 7   MTIP   Machine Timer Interrupt
//	 8   UEIP   User External Interrupt
//	 9   SEIP   Supervisor External Interrupt
//	10   —      reserved
//	11   MEIP   Machine External Interrupt
//	12   —      reserved (SGEIP in hypervisor ext)
//	13+  —      platform-defined / reserved
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

// IntString returns a string description of the pending interrupts in
// v.
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
