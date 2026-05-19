//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package cpu

import (
	"fmt"

	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/mmu"
)

type CSR int

func (csr CSR) ReadWrite() bool {
	return csr>>10&0b11 != 0b11
}

func (csr CSR) ReadOnly() bool {
	return csr>>10&0b11 == 0b11
}

func (csr CSR) Privilege() isa.PrivilegeMode {
	return isa.PrivilegeMode(csr >> 8 & 0b11)
}

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
	CsrStimecmp      = 0x14d // RISC-V Sstc (Supervisor Time Compare) extension
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

var csrs = map[int]string{
	0x003: "Fcsr",
	0x100: "Sstatus",
	0x104: "Sie",
	0x105: "Stvec",
	0x106: "Scounteren",
	0x140: "Sscratch",
	0x141: "Sepc",
	0x142: "Scause",
	0x143: "Stval",
	0x144: "Sip",
	0x14d: "Stimecmp",
	0x180: "Satp",
	0x300: "Mstatus",
	0x301: "Misa",
	0x302: "Medeleg",
	0x303: "Mideleg",
	0x304: "Mie",
	0x305: "Mtvec",
	0x306: "Mcounteren",
	0x30a: "Menvcfg",
	0x30c: "Mstateen0",
	0x320: "Mcountinhibit",
	0x321: "Mhpmevent3",
	0x340: "Mscratch",
	0x341: "Mepc",
	0x342: "Mcause",
	0x343: "Mtval",
	0x344: "Mip",
	0x3a0: "Mpmpcfg0",
	0x3b0: "Mpmpcfg2",
	0x3b1: "Mpmpaddr8",
	0x7a0: "Tselect",
	0x7a1: "Tdata1",
	0x7a4: "Tinfo",
	0xc00: "Cycle",
	0xc01: "Time",
	0xda0: "Senvcfg",
	0xf11: "Mvendorid",
	0xf12: "Marchid",
	0xf13: "Mimpid",
	0xf14: "Mhartid",
	0xfb0: "Scountinhibit",
}

func csrName(csr int) string {
	name, ok := csrs[csr]
	if ok {
		return fmt.Sprintf("%s[%03x]", name, csr)
	}
	return fmt.Sprintf("%03x", csr)
}

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

func (cpu *CPU) GetCSR(csr CSR) (uint64, error) {
	if cpu.Mode() < csr.Privilege() && false {
		return 0, cpu.Trap(isa.CauseIllegalInstr, uint64(csr),
			fmt.Errorf("GetCSR(%x), mode=%v", csr, cpu.Mode()))
	}
	// Handle read-only CSRs here by returning the fixed or computed
	// value.

	var v uint64

	switch csr {
	case CsrMstatus:
		v = uint64(cpu.mstatus)

	case CsrMisa:
		v = cpu.CSR[csr]
		v |= isa.MisaMXL |
			isa.MisaA | // A (Atomic)
			isa.MisaC | // C (Compressed)
			isa.MisaD | // D (Double)
			isa.MisaF | // F (Float)
			isa.MisaG | // G (Additional alias for IMAFD)
			isa.MisaI | // I (Integer)
			isa.MisaM | // M (Multiply)
			isa.MisaS | // S (Supervisor)
			isa.MisaU // U (User mode)

		// Debug triggers.
	case 0x7a0, 0x7a1, 0x7a2, 0x7a3, 0x7a4:

	case CsrTime:
		v = cpu.Now()

	// case CsrMvendorid:

	case CsrMarchid:
		v = 0x100

	case CsrMimpid:
		v = 0x1

	case CsrMhartid:

	case CsrScountinhibit:

	case CsrSstatus:
		v = uint64(cpu.mstatus & isa.SstatusMask)

	case CsrSie:
		mask := uint64((1 << 1) | (1 << 5) | (1 << 9))
		v = cpu.CSR[CsrMie] & mask

	case CsrSip:
		mask := uint64((1 << 1) | (1 << 5) | (1 << 9))
		v = cpu.CSR[CsrMip] & mask

	default:
		if csr >= 0xb03 && csr <= 0xb1f {
			// Mhpmcounters
		} else {
			v = cpu.CSR[csr]
		}
	}
	return v, nil
}

func (cpu *CPU) SetCSR(csr CSR, v uint64) error {
	return cpu.SetCSRX(csr, v, 0, isa.Instr{})
}

func (cpu *CPU) SetCSRX(csr CSR, v uint64, raw uint32, instr isa.Instr) error {

	if cpu.Mode() < csr.Privilege() && false {
		return cpu.Trap(isa.CauseIllegalInstr, uint64(csr),
			fmt.Errorf("SetCSR(%x)=%v, mode=%v", csr, v, cpu.Mode()))
	}
	if csr.ReadOnly() {
		return cpu.Trap(isa.CauseIllegalInstr, uint64(csr),
			fmt.Errorf("SetCSR(%x)=%v: read-only", csr, v))
	}

	// Handle read-only and functional CSRs here by ignoring update or
	// by updating CPU state accordingly.
	switch csr {
	case CsrMstatus:
		cpu.mstatus = isa.Mstatus(v)

	case CsrMisa:
		cpu.CSR[csr] = v

	case CsrMie, CsrMip:
		cpu.CSR[csr] = v

	case CsrSstatus:
		cpu.mstatus = (cpu.mstatus & ^isa.SstatusMask) |
			(isa.Mstatus(v) & isa.SstatusMask)

	case CsrStimecmp:
		cpu.CSR[csr] = v

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

		// Save Satp to CSR so that it can be queried.
		cpu.CSR[csr] = v

		if cpu.Trace {
			cpu.traceFunc(cpu.PC)
			cpu.tracef(raw, instr, "Satp: %v", satp)
		}
		if v != 0 {
			cpu.tracef(raw, instr, "Satp: %v", satp)
			cpu.DebugTrace = true
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

	return nil
}

func (cpu *CPU) Mstatus() isa.Mstatus {
	return cpu.mstatus
}
