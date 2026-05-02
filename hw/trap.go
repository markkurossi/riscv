//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package hw

import (
	"fmt"

	"github.com/markkurossi/riscv/isa"
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
	PC    uint64
	Tval  uint64
	Cause uint64
	Err   error
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
	return fmt.Sprintf("%s: pc=%x, tval=%x", name, trap.PC, trap.Tval)
}

func (trap *Trap) Unwrap() error {
	return trap.Err
}

func (cpu *CPU) Trap(cause, tval uint64, err error) error {
	cpu.Sepc = cpu.PC
	cpu.Scause = cause
	cpu.Stval = tval

	return &Trap{
		PC:    cpu.PC,
		Tval:  tval,
		Cause: cause,
		Err:   err,
	}
}

func (cpu *CPU) HandleTrap(trap *Trap) error {
	// XXX check if our trap handler can handle this. If not handled,
	// print the error message below and return the trap as error.

	fmt.Println(trap.Error())
	if trap.Err != nil {
		fmt.Printf("  caused by: %v\n", trap.Err)
	}
	cpu.Dump(trap.PC)

	return trap
}

func (cpu *CPU) Dump(epc uint64) {
	fmt.Printf("CPU: 0 PID: %v IC: %v\n", cpu.PID, cpu.Instret)
	fmt.Printf("epc : %016x ra : %016x sp : %016x\n",
		epc, cpu.X[isa.Ra], cpu.X[isa.Sp])
	fmt.Printf(" gp : %016x tp : %016x t0 : %016x\n",
		cpu.X[isa.Gp], cpu.X[isa.Tp], cpu.X[isa.T0])
	fmt.Printf(" t1 : %016x t2 : %016x s0 : %016x\n",
		cpu.X[isa.T1], cpu.X[isa.T2], cpu.X[isa.S0])
	fmt.Printf(" s1 : %016x a0 : %016x a1 : %016x\n",
		cpu.X[isa.S1], cpu.X[isa.A0], cpu.X[isa.A1])
	fmt.Printf(" a2 : %016x a3 : %016x a4 : %016x\n",
		cpu.X[isa.A2], cpu.X[isa.A3], cpu.X[isa.A4])
	fmt.Printf(" a5 : %016x a6 : %016x a7 : %016x\n",
		cpu.X[isa.A5], cpu.X[isa.A6], cpu.X[isa.A7])
	fmt.Printf(" s2 : %016x s3 : %016x s4 : %016x\n",
		cpu.X[isa.S2], cpu.X[isa.S3], cpu.X[isa.S4])
	fmt.Printf(" s5 : %016x s6 : %016x s7 : %016x\n",
		cpu.X[isa.S5], cpu.X[isa.S6], cpu.X[isa.S7])
	fmt.Printf(" s8 : %016x s9 : %016x s10: %016x\n",
		cpu.X[isa.S8], cpu.X[isa.S9], cpu.X[isa.S10])
	fmt.Printf(" s11: %016x t3 : %016x t4 : %016x\n",
		cpu.X[isa.S11], cpu.X[isa.T3], cpu.X[isa.T4])
	fmt.Printf(" t5 : %016x t6 : %016x\n",
		cpu.X[isa.T5], cpu.X[isa.T6])
}
