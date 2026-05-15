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

func (cpu *CPU) Trap(target isa.PrivilegeMode, cause, tval uint64,
	err error) *isa.Trap {

	switch target {
	case isa.ModeM:
		// 1. Get current mstatus
		mstatus := cpu.GetCSR(CsrMstatus)

		// 2. Save Current Mode (S=1) into MPP (bits 11-12)
		mstatus = (mstatus & ^uint64(0x1800)) | (uint64(cpu.Mode) << 11)

		// 3. Save MIE into MPIE, then disable MIE
		mie := (mstatus >> 3) & 0x1
		mstatus = (mstatus & ^uint64(1<<7)) | (mie << 7) // MPIE = MIE
		mstatus = (mstatus & ^uint64(1<<3))              // MIE = 0

		// 4. Update CSR and CPU state
		cpu.SetCSR(CsrMstatus, mstatus)
		cpu.SetCSR(CsrMepc, cpu.PC)
		cpu.SetCSR(CsrMcause, cause)
		cpu.SetCSR(CsrMtval, tval)

		return isa.NewTrap(target, cpu.PC, cause, tval, err)

	case isa.ModeS:
		status := cpu.GetCSR(CsrSstatus)
		// SPP is bit 8, not 11-12
		status = (status & ^uint64(1<<8)) | (uint64(cpu.Mode&1) << 8)
		// SPIE is bit 5, SIE is bit 1
		sie := (status >> 1) & 1
		status = (status & ^uint64(1<<5)) | (sie << 5)
		status &= ^uint64(1 << 1) // Disable SIE

		// 4. Update CSR and CPU state
		cpu.SetCSR(CsrSstatus, status)
		cpu.SetCSR(CsrSepc, cpu.PC)
		cpu.SetCSR(CsrScause, cause)
		cpu.SetCSR(CsrStval, tval)

		return isa.NewTrap(target, cpu.PC, cause, tval, err)

	default:
		return isa.NewTrap(isa.ModeM, cpu.PC, isa.CauseBreakpoint, 0,
			fmt.Errorf("unexpected target mode %v", target))
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
	fmt.Printf("CPU: 0 PID: %v IC: %v\n", cpu.PID, cpu.Instret)

	if cpu.Symtab != nil {
		entry := cpu.Symtab.Resolve(epc)
		if entry != nil {
			fmt.Printf("epc : %s+0x%x\n", entry.Name, epc-entry.Addr)
		}
		entry = cpu.Symtab.Resolve(cpu.X[isa.Ra])
		if entry != nil {
			fmt.Printf(" ra : %s+0x%x\n", entry.Name, cpu.X[isa.Ra]-entry.Addr)
		}
	}

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

	fmt.Printf("Satp: mode=%v, page=%x\n",
		cpu.MMU.Satp().Mode(), cpu.MMU.Satp().PPN())

	page, err := cpu.MMU.Mem.Page(uint64(cpu.MMU.Satp().PPN()))
	if err != nil {
		fmt.Printf("Page table root not found: %v\n", err)
	} else if false {
		fmt.Printf("Page table root:\n%s", hex.Dump(page))
	}
}
