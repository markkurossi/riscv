//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package cpu

import (
	"github.com/markkurossi/riscv/mmu"
)

type CSR int

const (
	CsrFcsr          = 0x003
	CsrSstatus       = 0x100
	CsrSie           = 0x104
	CsrStvec         = 0x105
	CsrScounteren    = 0x106
	CsrSscratch      = 0x140
	CsrSepc          = 0x141
	CsrScause        = 0x142
	CsrStval         = 0x143
	CsrSip           = 0x144
	CsrScontext      = 0x14d
	CsrSatp          = 0x180
	CsrMstatus       = 0x300
	CsrMisa          = 0x301
	CsrMedeleg       = 0x302
	CsrMideleg       = 0x303
	CsrMie           = 0x304
	CsrMtvec         = 0x305
	CsrMcounteren    = 0x306
	CsrMenvcfg       = 0x30a
	CsrMstateen0     = 0x30c
	CsrMcountinhibit = 0x320
	CsrMhpmevent3    = 0x321
	CsrMscratch      = 0x340
	CsrMepc          = 0x341
	CsrMcause        = 0x342
	CsrMtval         = 0x343
	CsrMip           = 0x344
	CsrMpmpcfg0      = 0x3a0
	CsrMpmpcfg2      = 0x3b0
	CsrMpmpaddr8     = 0x3b1
	CsrTselect       = 0x7a0
	CsrTdata1        = 0x7a1
	CsrTinfo         = 0x7a4
	CsrCycle         = 0xc00
	CsrTime          = 0xc01
	CsrSenvcfg       = 0xda0
	CsrMvendorid     = 0xf11
	CsrMarchid       = 0xf12
	CsrMimpid        = 0xf13
	CsrMhartid       = 0xf14
	CsrScountinhibit = 0xfb0
)

// CsrMedeleg:
//
//  func (cpu *CPU) trap(exceptionCode uint64) {
//      // If we are in S or U mode, and the bit is set in medeleg...
//      if cpu.Mode <= ModeS && (cpu.M.edeleg & (1 << exceptionCode)) != 0 {
//          // Jump to stvec (Supervisor Trap Vector)
//          cpu.transferToSMode(exceptionCode)
//      } else {
//          // Jump to mtvec (Machine Trap Vector)
//          cpu.transferToMMode(exceptionCode)
//      }
//  }
//
// CsrMideleg: Simplified interrupt logic
//
//  if cpu.Mode < ModeM && (cpu.M.ideleg & (1 << interruptID)) != 0 {
//      // Jump to stvec (Supervisor Trap Vector)
//      cpu.TrapToSMode(interruptID)
//  } else {
//      // Jump to mtvec (Machine Trap Vector)
//      cpu.TrapToMMode(interruptID)
//  }

func (cpu *CPU) LoadCSR(csr CSR) uint64 {
	// Handle read-only CSRs here by returning the fixed or computed
	// value.
	switch csr {
	case CsrMisa:
		// RV64IMAFDC
		return uint64(2<<62) | // MXLEN = 64
			(1 << 0) | // A (Atomic)
			(1 << 2) | // C (Compressed)
			(1 << 3) | // D (Double)
			(1 << 5) | // F (Float)
			(1 << 8) | // I (Integer)
			(1 << 12) | // M (Multiply)
			(1 << 18) | // S (Supervisor)
			(1 << 20) // U (User mode)

		// Debug triggers.
	case 0x7a0, 0x7a1, 0x7a2, 0x7a3, 0x7a4:
		return 0

	case CsrTime:
		return cpu.Instret

	case CsrMvendorid, CsrMarchid, CsrMimpid:
		return 0

	case CsrMhartid:
		return 0

	case CsrScountinhibit:
		return 0

	default:
		if csr >= 0xb03 && csr <= 0xb1f {
			// Mhpmcounters
			return 0
		}
		return cpu.CSR[csr]
	}
}

func (cpu *CPU) StoreCSR(csr CSR, v uint64) {
	// Handle read-only and functional CSRs here by ignoring update or
	// by updating CPU state accordingly.
	switch csr {
	case CsrMisa:

	case CsrSatp:
		// XXX flush TLB?
		cpu.MMU.Satp = mmu.Satp(v)

	default:
		cpu.CSR[csr] = v
	}
}
