//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package cpu

//lint:file-ignore ST1003 to match the CSR naming conventions.

import (
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"runtime/pprof"
	"time"

	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/memory"
	"github.com/markkurossi/riscv/mmu"
)

const (
	debugCSR = false
)

func (cpu *CPU) CSRLoad(csr isa.CSR, raw uint32, instr isa.Instr) (
	uint64, error) {

	if cpu.Mode() < csr.Privilege() {
		return 0, cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
			fmt.Errorf("GetCSR(%v), mode=%v", csr, cpu.Mode()))
	}
	// Handle read-only CSRs here by returning the fixed or computed
	// value.

	var v uint64

	switch csr {
	case isa.CsrFflags, isa.CsrFrm, isa.CsrFcsr:
		if cpu.mstatus.FS() == isa.RegOff && checkFSRegOff {
			return 0, cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("read fcsr when FS is off"))
		}
		v := cpu.CSR[csr].Load()
		switch csr {
		case isa.CsrFflags:
			v &= 0b11111
		case isa.CsrFrm:
			v = v >> 5 & 0b111
		}

	case isa.CsrMstatus:
		v = uint64(cpu.mstatus)

	case isa.CsrMisa:
		v = cpu.CSR[csr].Load()
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

	case isa.CsrCycle:
		v = cpu.Time

	case isa.CsrTime:
		v = cpu.syncTime()

	case isa.CsrInstret:
		v = cpu.Instret

	case isa.CsrMvendorid:

	case isa.CsrMarchid:
		v = 0x100

	case isa.CsrMimpid:
		v = 0x1

	case isa.CsrMhartid:

	case isa.CsrScountinhibit:

	case isa.CsrSstatus:
		v = uint64(cpu.mstatus & isa.SstatusMask)

	case isa.CsrSie:
		mask := uint64(isa.IntSSIP | isa.IntSTIP | isa.IntSEIP)
		v = cpu.CSR[isa.CsrMie].Load() & mask

	case isa.CsrSip:
		mask := uint64(isa.IntSSIP | isa.IntSTIP | isa.IntSEIP)
		v = cpu.CSR[isa.CsrMip].Load() & mask

	case isa.CsrSatp:
		if cpu.mstatus.TVM() {
			return 0, cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("get satp with TVM set"))
		}
		v = cpu.CSR[csr].Load()

		// Vector extension.

	case isa.CsrVstart:
		v = cpu.vpu.VStart

	case isa.CsrVl:
		v = cpu.vpu.VL

	case isa.CsrVtype:
		v = uint64(cpu.vpu.VType)

	case isa.CsrVlenb:
		v = uint64(cpu.vpu.VLEN / 8)

	case isa.CsrSeed:
		var buf [2]byte
		_, err := rand.Read(buf[:])
		if err != nil {
			return 0, err
		}
		v = uint64(0x80000000)
		v |= uint64(buf[0]) << 8
		v |= uint64(buf[1])

	case isa.CsrGoemuTime:
		v = uint64(time.Now().UnixNano())

	default:
		if csr >= 0xb03 && csr <= 0xb1f {
			// Mhpmcounters
		} else {
			v = cpu.CSR[csr].Load()
		}
	}

	if debugCSR {
		log.Printf("GetCSR(%v): %v", csr, v)
	}

	return v, nil
}

func (cpu *CPU) SetCSR(csr isa.CSR, v uint64) error {
	return cpu.SetCSRX(csr, v, 0, isa.Instr{})
}

func (cpu *CPU) SetCSRX(csr isa.CSR, v uint64, raw uint32,
	instr isa.Instr) error {

	if debugCSR {
		log.Printf("SetCSR(%v, %v)", csr, v)
	}

	if cpu.Mode() < csr.Privilege() && false {
		return cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
			fmt.Errorf("SetCSR(%v)=%v, mode=%v", csr, v, cpu.Mode()))
	}
	if csr.ReadOnly() {
		return cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
			fmt.Errorf("SetCSR(%v)=%v: read-only", csr, v))
	}

	// Handle read-only and functional CSRs here by ignoring update or
	// by updating CPU state accordingly.
	switch csr {
	case isa.CsrFflags, isa.CsrFrm, isa.CsrFcsr:
		if cpu.mstatus.FS() == isa.RegOff && checkFSRegOff {
			return cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("write fcsr when FS is off"))
		}
		switch csr {
		case isa.CsrFflags:
			n := cpu.CSR[csr].Load()
			n &= ^uint64(0b11111)
			n |= v & 0b11111
			cpu.CSR[csr].Store(n)

		case isa.CsrFrm:
			n := cpu.CSR[csr].Load()
			n &= ^uint64(0b11100000)
			n |= (v & 0b111) << 5
			cpu.CSR[csr].Store(n)

		default:
			cpu.CSR[csr].Store(v)
		}

	case isa.CsrMstatus:
		cpu.mstatus = cpu.mstatus&isa.MConstMask |
			isa.Mstatus(v)&^isa.MConstMask
		cpu.updateMstatusSD()

	case isa.CsrMisa:
		cpu.CSR[csr].Store(v)

	case isa.CsrMie, isa.CsrMip:
		cpu.CSR[csr].Store(v)

	case isa.CsrSstatus:
		cpu.mstatus = (cpu.mstatus & ^isa.SstatusMask) |
			(isa.Mstatus(v) & isa.SstatusMask)
		cpu.updateMstatusSD()

	case isa.CsrStimecmp:
		cpu.CSR[csr].Store(v)
		if cpu.syncTime() < v {
			// Next timer interrupt in the future, clear interrupts.
			cpu.CSR[isa.CsrMip].And(^uint64(isa.IntSTIP))
		}

	case isa.CsrSie:
		// sie is a masked view of mie — only S-mode bits
		mask := uint64(isa.IntSSIP | isa.IntSTIP | isa.IntSEIP)
		mie := cpu.CSR[isa.CsrMie].Load()
		cpu.CSR[isa.CsrMie].Store((mie & ^mask) | (v & mask))

	case isa.CsrSip:
		// sip is a masked view of mip — only S-mode bits.
		mask := uint64(isa.IntSSIP | isa.IntSTIP | isa.IntSEIP)
		mip := cpu.CSR[isa.CsrMip].Load()
		cpu.CSR[isa.CsrMip].Store((mip & ^mask) | (v & mask))

	case isa.CsrSatp:
		if cpu.mstatus.TVM() {
			return cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("set satp with TVM set"))
		}
		satp := mmu.Satp(v)
		cpu.MMU.SetSatp(satp)
		cpu.codePagenum = memory.InvalidPagenum
		cpu.codePage = nil

		// Save Satp to CSR so that it can be queried.
		cpu.CSR[csr].Store(v)

		if cpu.Trace {
			cpu.traceFunc(cpu.PC)
			cpu.tracef(raw, instr, "Satp: %v", satp)
		}

	case isa.CsrGoemuDebug:
		cpu.DebugTrace = v&0b1 != 0
		cpu.CSR[csr].Store(v)

	case isa.CsrGoemuCPUProfile:
		if v == 0 {
			cpu.csr802Refcount--
			if cpu.csr802Refcount <= 0 {
				pprof.StopCPUProfile()
				cpu.csr802File.Sync()
			}
		} else {
			if cpu.csr802Refcount == 0 {
				var err error
				if cpu.csr802File == nil {
					cpu.csr802File, err = os.Create(cpu.CSR802Filename)
					if err != nil {
						return err
					}
				}
				err = pprof.StartCPUProfile(cpu.csr802File)
				if err != nil {
					return err
				}
			}
			cpu.csr802Refcount++
		}

	default:
		cpu.CSR[csr].Store(v)
	}

	return nil
}

func (cpu *CPU) updateMstatusSD() {
	if cpu.mstatus.VS() == isa.RegDirty || cpu.mstatus.FS() == isa.RegDirty {
		cpu.mstatus.SetSD(true)
	} else {
		cpu.mstatus.SetSD(false)
	}
}

func (cpu *CPU) Mstatus() isa.Mstatus {
	return cpu.mstatus
}
