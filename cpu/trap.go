//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package cpu

import (
	"encoding/hex"
	"fmt"

	"github.com/markkurossi/riscv/isa"
)

func (cpu *CPU) Trap(cause, tval uint64, err error) error {
	epc := cpu.PC

	if ierr := cpu.trap(epc, cause, tval, err); ierr != nil {
		return ierr
	}

	return isa.NewTrap(epc, cause, tval, err)
}

func (cpu *CPU) trap(epc, cause, tval uint64, err error) error {

	cpu.ReservationValid = false

	if cpu.TrapHandler != nil {
		t := isa.NewTrap(epc, cause, tval, nil)
		handled, err := cpu.TrapHandler(cpu, t)
		if err != nil {
			return err
		}
		if handled {
			return nil
		}
		return t
	}

	// Handler is determined by the medeleg (Machine Exception
	// Delegation) register.
	var tvec uint64
	medeleg := cpu.CSR[CsrMedeleg]
	if medeleg&(1<<cause) == 0 || cpu.Mode() == isa.ModeM {
		// Trap to M-mode.
		cpu.mstatus.SetMPP(cpu.Mode())
		cpu.mstatus.SetMPIE(cpu.mstatus.MIE())
		cpu.mstatus.SetMIE(false)

		cpu.CSR[CsrMepc] = epc
		cpu.CSR[CsrMcause] = cause
		cpu.CSR[CsrMtval] = tval

		tvec = cpu.CSR[CsrMtvec]
		cpu.SetMode(isa.ModeM)
	} else {
		// Delegated to S-mode.
		cpu.mstatus.SetSPP(cpu.Mode())
		cpu.mstatus.SetSPIE(cpu.mstatus.SIE())
		cpu.mstatus.SetSIE(false)

		cpu.CSR[CsrSepc] = epc
		cpu.CSR[CsrScause] = cause
		cpu.CSR[CsrStval] = tval

		tvec = cpu.CSR[CsrStvec]
		cpu.SetMode(isa.ModeS)
	}

	mode := tvec & 0x3
	base := tvec &^ 0x3

	// Bit 63 indicates an asynchronous interrupt in RISC-V
	isInterrupt := (cause >> 63) == 1

	if mode == 1 && isInterrupt {
		// Isolate the true cause index (clear the MSB interrupt flag)
		causeIdx := cause &^ (uint64(1) << 63)
		cpu.PC = base + (causeIdx * 4)
	} else {
		cpu.PC = base
	}

	return nil
}

func (cpu *CPU) Interrupt(target isa.PrivilegeMode, cause uint64) {

	cause |= 1 << 63

	var tvec uint64

	epc := cpu.PC
	switch target {
	case isa.ModeM:
		cpu.mstatus.SetMPP(cpu.Mode())
		cpu.mstatus.SetMPIE(cpu.mstatus.MIE())
		cpu.mstatus.SetMIE(false)

		cpu.CSR[CsrMepc] = epc
		cpu.CSR[CsrMcause] = cause
		cpu.CSR[CsrMtval] = 0

		tvec = cpu.CSR[CsrMtvec]
		cpu.SetMode(isa.ModeM)

	case isa.ModeS:
		cpu.mstatus.SetSPP(cpu.Mode())
		cpu.mstatus.SetSPIE(cpu.mstatus.SIE())
		cpu.mstatus.SetSIE(false)

		cpu.CSR[CsrSepc] = epc
		cpu.CSR[CsrScause] = cause
		cpu.CSR[CsrStval] = 0

		tvec = cpu.CSR[CsrStvec]
		cpu.SetMode(isa.ModeS)

	default:
		panic("invalid interrupt target")
	}

	mode := tvec & 0x3
	base := tvec &^ 0x3

	if mode == 1 {
		// Isolate the true cause index (clear the MSB interrupt flag)
		causeIdx := cause &^ (uint64(1) << 63)
		cpu.PC = base + (causeIdx * 4)
	} else {
		cpu.PC = base
	}
}

func (cpu *CPU) HandleTrap(trap *isa.Trap) error {
	if cpu.TrapHandler != nil {
		ok, err := cpu.TrapHandler(cpu, trap)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}

	fmt.Println(trap.Error())
	if trap.Err != nil {
		fmt.Printf("  caused by: %v\n", trap.Err)
	}
	cpu.Dump(trap.PC)

	return trap
}

func (cpu *CPU) Dump(epc uint64) {
	fmt.Printf("CPU: 0 Mode: %v IC: %v Mstatus: %x\r\n",
		cpu.mode, cpu.Instret, cpu.mstatus)

	if cpu.Symtab != nil {
		entry, mapped := cpu.FuncName(epc)
		if entry != nil {
			fmt.Printf("epc : %s+0x%x\r\n", entry.Name, mapped-entry.Start)
		}
		entry, mapped = cpu.FuncName(cpu.X[isa.Ra])
		if entry != nil {
			fmt.Printf(" ra : %s+0x%x\r\n", entry.Name, mapped-entry.Start)
		}
	}

	fmt.Printf("epc : %016x ra : %016x sp : %016x\r\n",
		epc, cpu.X[isa.Ra], cpu.X[isa.Sp])
	fmt.Printf(" gp : %016x tp : %016x t0 : %016x\r\n",
		cpu.X[isa.Gp], cpu.X[isa.Tp], cpu.X[isa.T0])
	fmt.Printf(" t1 : %016x t2 : %016x s0 : %016x\r\n",
		cpu.X[isa.T1], cpu.X[isa.T2], cpu.X[isa.S0])
	fmt.Printf(" s1 : %016x a0 : %016x a1 : %016x\r\n",
		cpu.X[isa.S1], cpu.X[isa.A0], cpu.X[isa.A1])
	fmt.Printf(" a2 : %016x a3 : %016x a4 : %016x\r\n",
		cpu.X[isa.A2], cpu.X[isa.A3], cpu.X[isa.A4])
	fmt.Printf(" a5 : %016x a6 : %016x a7 : %016x\r\n",
		cpu.X[isa.A5], cpu.X[isa.A6], cpu.X[isa.A7])
	fmt.Printf(" s2 : %016x s3 : %016x s4 : %016x\r\n",
		cpu.X[isa.S2], cpu.X[isa.S3], cpu.X[isa.S4])
	fmt.Printf(" s5 : %016x s6 : %016x s7 : %016x\r\n",
		cpu.X[isa.S5], cpu.X[isa.S6], cpu.X[isa.S7])
	fmt.Printf(" s8 : %016x s9 : %016x s10: %016x\r\n",
		cpu.X[isa.S8], cpu.X[isa.S9], cpu.X[isa.S10])
	fmt.Printf(" s11: %016x t3 : %016x t4 : %016x\r\n",
		cpu.X[isa.S11], cpu.X[isa.T3], cpu.X[isa.T4])
	fmt.Printf(" t5 : %016x t6 : %016x\r\n",
		cpu.X[isa.T5], cpu.X[isa.T6])

	if false {
		fmt.Printf("Satp: %v\r\n", cpu.MMU.Satp())

		root := cpu.MMU.Satp().PPN()
		if root != 0 {
			page, err := cpu.MMU.Mem.Page(uint64(cpu.MMU.Satp().PPN()))
			if err != nil {
				fmt.Printf("Page table root not found: %v\r\n", err)
			} else {
				fmt.Printf("Page table root:\r\n%s", hex.Dump(page))
			}
		}
	}
}
