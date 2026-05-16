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
)

var causes = map[uint64]string{
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
}

type Trap struct {
	Target PrivilegeMode
	PC     uint64
	Tval   uint64
	Cause  uint64
	Err    error
}

func NewTrap(target PrivilegeMode, pc, cause, tval uint64, err error) *Trap {
	if false {
		fmt.Printf("Trap: pc=%x, cause=%v, tval=%x, err=%v\n",
			pc, cause, tval, err)
		if false {
			debug.PrintStack()
			os.Exit(1)
		}
	}
	return &Trap{
		Target: target,
		PC:     pc,
		Tval:   tval,
		Cause:  cause,
		Err:    err,
	}
}

func (trap *Trap) Error() string {
	if trap.Cause>>63 != 0 {
		return fmt.Sprintf("Interrupt %x: pc=%x, tval=%x",
			trap.Cause, trap.PC, trap.Tval)
	}
	name, ok := causes[trap.Cause]
	if !ok {
		name = fmt.Sprintf("{Cause %d}", trap.Cause)
	}
	return fmt.Sprintf("%s: target=%v, pc=%x, tval=%x",
		name, trap.Target, trap.PC, trap.Tval)
}

func (trap *Trap) Unwrap() error {
	return trap.Err
}
