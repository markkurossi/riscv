//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package cpu

import (
	"fmt"
	"os"
	"time"

	"github.com/markkurossi/riscv/isa"
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

var count int

func (cpu *CPU) GetCSR(csr CSR) uint64 {
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
		now := cpu.Instret

		// 1. Get the comparison value set by OpenSBI/Linux CLINT
		// mtimecmp is typically at CLINTBase + 0x4000
		// mtimecmp := cpu.GetMtimecmp() XXX
		mtimecmp := cpu.LastTimer + 100
		if mtimecmp < 100000000 {
			mtimecmp = 100000000
		}

		if cpu.Mode == isa.ModeS && now >= mtimecmp &&
			time.Now().Sub(cpu.StartTime) > time.Second*1 {

			fmt.Printf("***** CsrTime: a0=%v, time=%v\n", cpu.X[isa.A0], now)
			// cpu.DebugTrace = true
			count++
			if count > 5 {
				// cpu.MMU.Dump()
				cpu.Dump(cpu.PC)
				os.Exit(1)
			}
			fmt.Printf("mip=%064b\nmie=%064b\n",
				cpu.GetCSR(CsrMip),
				cpu.GetCSR(CsrMie))

			// 2. Set the "Timer Interrupt Pending" bit in mip
			// Bit 7 is MTIP (Machine Timer Interrupt Pending)
			// Bit 5 is STIP (Supervisor Timer Interrupt Pending)
			mip := cpu.GetCSR(CsrMip)
			cpu.SetCSR(CsrMip, mip|(1<<7)|(1<<5))

			// 3. Optional: If you want to force an immediate trap you
			// can trigger your handleTrap logic here if (status.MIE
			// && mip.MTIP)
			cpu.InterruptsPending = true
			cpu.LastTimer = now
		}

		return now

	case CsrMvendorid, CsrMarchid, CsrMimpid:
		return 0

	case CsrMhartid:
		return 0

	case CsrScountinhibit:
		return 0

	case CsrSstatus:
		mask := uint64(0x000de762)
		return cpu.CSR[CsrMstatus] & mask

	case CsrSie:
		mask := uint64((1 << 1) | (1 << 5) | (1 << 9))
		return cpu.CSR[CsrMie] & mask

	case CsrSip:
		mask := uint64((1 << 1) | (1 << 5) | (1 << 9))
		return cpu.CSR[CsrMip] & mask

	default:
		if csr >= 0xb03 && csr <= 0xb1f {
			// Mhpmcounters
			return 0
		}
		return cpu.CSR[csr]
	}
}

func (cpu *CPU) SetCSR(csr CSR, v uint64) {
	cpu.SetCSRX(csr, v, 0, isa.Instr{})
}

func (cpu *CPU) SetCSRX(csr CSR, v uint64, raw uint32, instr isa.Instr) {
	// Handle read-only and functional CSRs here by ignoring update or
	// by updating CPU state accordingly.
	switch csr {
	case CsrMisa:

	case CsrSstatus:
		mask := uint64(0x000de762) // S-mode visible bits of mstatus
		cpu.CSR[CsrMstatus] = (cpu.CSR[CsrMstatus] & ^mask) | (v & mask)

	case CsrSie:
		// sie is a masked view of mie — only S-mode bits
		mask := uint64((1 << 1) | (1 << 5) | (1 << 9))
		mie := cpu.CSR[CsrMie]
		cpu.CSR[CsrMie] = (mie & ^mask) | (v & mask)

	case CsrSip:
		// sip is a masked view of mip — only SSIP (bit 1) is writable by S-mode
		mask := uint64(1 << 1)
		mip := cpu.CSR[CsrMip]
		cpu.CSR[CsrMip] = (mip & ^mask) | (v & mask)

	case CsrSatp:
		satp := mmu.Satp(v)
		cpu.MMU.SetSatp(satp)
		if cpu.Trace {
			cpu.funcName(cpu.PC)
			cpu.tracef(raw, instr, "Satp: %v", satp)
		}
		if v != 0 {
			if false {
				cpu.DebugTrace = true
				cpu.MMU.Dump()
			}
		}

	default:
		cpu.CSR[csr] = v
	}
}
