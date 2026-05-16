//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

// Package cpu implements the virtual RISC-V CPU.
package cpu

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"os"
	"time"

	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/memory"
	"github.com/markkurossi/riscv/mmu"
)

var (
	bo = binary.LittleEndian
)

const (
	cpuDebug = false
)

type TrapHandler func(cpu *CPU, trap *isa.Trap) (bool, error)

type CPU struct {
	PID uint64 // XXX how is this set? This is an usermode emulator field.

	Trace bool

	X [32]uint64
	F [32]float64

	mode              isa.PrivilegeMode
	InterruptsPending bool
	LastTimer         uint64

	CSR [4096]uint64

	PC uint64

	Reservation    uint64 // The address currently reserved
	HasReservation bool   // Whether the reservation is active

	decodeCache [4096]struct {
		Raw   uint32
		Instr isa.Instr
	}

	// Instruction count
	Instret uint64

	MMU *mmu.MMU

	TrapHandler TrapHandler
	Symtab      Symtab

	lastDescOp isa.Op
	StartTime  time.Time
	DebugTrace bool
	LastSymbol *SymEntry
}

func (cpu *CPU) Mode() isa.PrivilegeMode {
	return cpu.mode
}

func (cpu *CPU) SetMode(mode isa.PrivilegeMode) {
	cpu.mode = mode
	cpu.MMU.Mode = mode
}

func (cpu *CPU) Run() error {
	cpu.StartTime = time.Now()

	for {
		err := cpu.loop()
		if err != nil {
			if trap, ok := errors.AsType[*isa.Trap](err); ok {
				if trap.Target == 0 {
					medeleg := cpu.GetCSR(CsrMedeleg)
					delegated := medeleg&(1<<trap.Cause) != 0

					if true {
						fmt.Printf("%8x: cause=%v, mode=%v, medeleg=%v[%v]\n",
							cpu.PC, trap.Cause, cpu.Mode(), medeleg, delegated)
					}

					var target isa.PrivilegeMode

					if delegated && cpu.Mode() <= isa.ModeS {
						target = isa.ModeS
					} else {
						target = isa.ModeM
					}

					trap = cpu.Trap(target, trap.Cause, trap.Tval, trap.Err)
					if cpu.Trace {
						cpu.funcName(cpu.PC)
					}
				}
				err = cpu.HandleTrap(trap)
			}
			if err != nil {
				return err
			}
			// Exception handled, let's continue
		}
	}
}

func (cpu *CPU) loop() error {
	if cpu.PC%2 == 1 {
		return isa.NewTrap(0, cpu.PC, isa.CauseInstAddrMisaligned, cpu.PC, nil)
	}

	var codePagenum uint64
	var codePage []byte

	for {
		var instr isa.Instr
		var err error
		var size int

		cpu.X[isa.Zero] = 0

		// Check interrupts every 64 instructions.
		if cpu.Instret&0x3f == 0 {
			mip := cpu.GetCSR(CsrMip)
			mie := cpu.GetCSR(CsrMie)
			pending := mip & mie

			if pending != 0 {
				mstatus := cpu.GetCSR(CsrMstatus)
				mideleg := cpu.GetCSR(CsrMideleg)

				// Check each pending interrupt, highest priority first
				for _, bit := range []uint64{11, 9, 7, 5, 3, 1} {
					if pending&(1<<bit) == 0 {
						continue
					}
					if mideleg&(1<<bit) != 0 && cpu.Mode() != isa.ModeM {
						// Delegated to S-mode
						sie := (mstatus >> 1) & 1 // sstatus.SIE = mstatus bit 1
						if sie == 1 || cpu.Mode() < isa.ModeS {
							cause := 1<<63 | bit
							return cpu.Trap(isa.ModeS, cause, 0, nil)
						}
					} else {
						// Handle in M-mode
						mie_bit := (mstatus >> 3) & 1 // mstatus.MIE
						if mie_bit == 1 || cpu.Mode() < isa.ModeM {
							cause := 1<<63 | bit
							return cpu.Trap(isa.ModeM, cause, 0, nil)
						}
					}
				}
			}
		}

		if memory.Page(cpu.PC) != codePagenum {
			paddr, err := cpu.MMU.Map(cpu.PC, mmu.AccessExec)
			if err != nil {
				return err
			}
			codePage, err = cpu.MMU.Mem.Page(memory.Page(paddr))
			if err != nil {
				return err
			}
			codePagenum = memory.Page(cpu.PC)
		}
		ofs := memory.PageOffset(cpu.PC)
		raw := uint32(codePage[ofs]) | uint32(codePage[ofs+1])<<8

		if raw&0b11 == 0b11 {
			// 32-bit instruction.
			if cpu.PC>>12 == (cpu.PC+2)>>12 {
				// Same page.
				raw |= uint32(codePage[ofs+2]) << 16
				raw |= uint32(codePage[ofs+3]) << 24
			} else {
				// 32-bit instruction crosses page boundary.
				paddr, err := cpu.MMU.Map(cpu.PC+2, mmu.AccessExec)
				if err != nil {
					return err
				}
				nextPage, err := cpu.MMU.Mem.Page(memory.Page(paddr))
				if err != nil {
					return err
				}
				nextOfs := memory.PageOffset(paddr)
				raw |= uint32(nextPage[nextOfs+0]) << 16
				raw |= uint32(nextPage[nextOfs+1]) << 24
			}
			size = 4

			idx := (raw >> 2) & 0xfff
			if cpu.decodeCache[idx].Raw == raw {
				instr = cpu.decodeCache[idx].Instr
			} else {
				instr, err = isa.Decode(raw)
				if err != nil {
					return err
				}
				cpu.decodeCache[idx].Raw = raw
				cpu.decodeCache[idx].Instr = instr
			}
		} else {
			size = 2
			instr = isa.DecodeCFast(uint16(raw))
		}
		if err != nil {
			return isa.NewTrap(0, cpu.PC, isa.CauseIllegalInstr, uint64(raw),
				err)
		}
		cpu.Instret++

		if cpuDebug || cpu.DebugTrace {
			cpu.trace(raw, instr, "")
		}
		if (cpuDebug || cpu.DebugTrace) && cpu.Symtab != nil {
			print := true
			mapped := cpu.PC

			// OpenSBI range: no kernel symbols here
			if mapped >= 0x80000000 && mapped < 0x80200000 {
				print = false
			}
			if mapped > 0x80200000 && mapped < 0x100000000 {
				// Physical address during early boot (MMU off).
				// Kernel is loaded at 0x80200000 physical = 0xffffffff80000000 virtual.
				// delta = 0xffffffff80000000 - 0x80200000 = 0xffffffff7fe00000
				mapped = mapped - 0x200000 + 0xffffffff00000000
			}
			entry := cpu.Symtab.Resolve(mapped)
			if print && entry != nil && entry != cpu.LastSymbol {
				fmt.Printf("%8x:  %s+0x%x\n",
					cpu.PC, entry.Name, mapped-entry.Addr)
				cpu.LastSymbol = entry
			}
			if print && entry.Name == "__delay" {
				os.Exit(1)
			}
		}

		switch instr.Op {
		case isa.Add:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] + cpu.X[instr.Rs2]

		case isa.Addw:
			cpu.X[instr.Rd] = uint64(int64(int32(cpu.X[instr.Rs1]) +
				int32(cpu.X[instr.Rs2])))

		case isa.Addi:
			cpu.X[instr.Rd] = uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))

		case isa.Addiw:
			cpu.X[instr.Rd] = uint64(int64(int32(int64(cpu.X[instr.Rs1]) +
				int64(instr.Imm))))

		case isa.And:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] & cpu.X[instr.Rs2]

		case isa.Andi:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] & uint64(instr.Imm)

		case isa.Auipc:
			cpu.X[instr.Rd] = uint64(int64(cpu.PC) + int64(instr.Imm))

		case isa.Beq:
			if cpu.X[instr.Rs1] == cpu.X[instr.Rs2] {
				cpu.PC = uint64(int64(cpu.PC) + int64(instr.Imm))
				continue
			}

		case isa.Bge:
			if int64(cpu.X[instr.Rs1]) >= int64(cpu.X[instr.Rs2]) {
				cpu.PC = uint64(int64(cpu.PC) + int64(instr.Imm))
				continue
			}

		case isa.Bgeu:
			if cpu.X[instr.Rs1] >= cpu.X[instr.Rs2] {
				cpu.PC = uint64(int64(cpu.PC) + int64(instr.Imm))
				continue
			}

		case isa.Blt:
			if int64(cpu.X[instr.Rs1]) < int64(cpu.X[instr.Rs2]) {
				cpu.PC = uint64(int64(cpu.PC) + int64(instr.Imm))
				continue
			}

		case isa.Bltu:
			if cpu.X[instr.Rs1] < cpu.X[instr.Rs2] {
				cpu.PC = uint64(int64(cpu.PC) + int64(instr.Imm))
				continue
			}

		case isa.Bne:
			if cpu.X[instr.Rs1] != cpu.X[instr.Rs2] {
				cpu.PC = uint64(int64(cpu.PC) + int64(instr.Imm))
				continue
			}

		case isa.Div:
			if cpu.X[instr.Rs2] == 0 {
				// Division by zero → result = -1
				cpu.X[instr.Rd] = ^uint64(0)
			} else {
				cpu.X[instr.Rd] = uint64(int64(cpu.X[instr.Rs1]) /
					int64(cpu.X[instr.Rs2]))
			}

		case isa.Divu:
			if cpu.X[instr.Rs2] == 0 {
				// Division by zero → result = -1
				cpu.X[instr.Rd] = ^uint64(0)
			} else {
				cpu.X[instr.Rd] = cpu.X[instr.Rs1] / cpu.X[instr.Rs2]
			}

		case isa.Divuw:
			if cpu.X[instr.Rs2] == 0 {
				// Division by zero → result = -1
				cpu.X[instr.Rd] = ^uint64(0)
			} else {
				cpu.X[instr.Rd] = uint64(int64(int32(uint32(cpu.X[instr.Rs1]) /
					uint32(cpu.X[instr.Rs2]))))
			}

		case isa.Divw:
			if cpu.X[instr.Rs2] == 0 {
				// Division by zero → result = -1
				cpu.X[instr.Rd] = ^uint64(0)
			} else {
				cpu.X[instr.Rd] = uint64(int64(int32(cpu.X[instr.Rs1]) /
					int32(cpu.X[instr.Rs2])))
			}

		case isa.Ebreak:
			if cpu.Trace {
				cpu.funcName(cpu.PC)
				cpu.tracef(raw, instr, "")
			}

			// XXX This is determined by the medeleg (Machine
			// Exception Delegation) register.
			//
			//  - If medeleg[3] (Breakpoint bit) is 1: Trap to S-mode.
			//  - If medeleg[3] is 0: Trap to M-mode.

			return cpu.Trap(isa.ModeM, isa.CauseBreakpoint, 0, nil)

		case isa.Sret:
			status := cpu.GetCSR(CsrSstatus)
			// privilege ← SPP (bit 8)
			spp := (status >> 8) & 1
			cpu.SetMode(isa.PrivilegeMode(spp))

			// SIE ← SPIE (bit 5)
			spie := (status >> 5) & 1
			status = (status & ^uint64(1<<1)) | (spie << 1)
			// SPIE ← 1
			status |= (1 << 5)
			// SPP ← U (0)
			status &= ^uint64(1 << 8)
			cpu.SetCSR(CsrSstatus, status)
			cpu.PC = cpu.GetCSR(CsrSepc)
			if cpu.Trace {
				cpu.tracef(raw, instr, "sepc=%x", cpu.PC)
			}
			continue

		case isa.Mret:
			mstatus := cpu.GetCSR(CsrMstatus)

			// 1. Restore Mode from MPP
			mpp := (mstatus >> 11) & 0x3
			cpu.SetMode(isa.PrivilegeMode(mpp))

			// 2. Restore Interrupts: MIE = MPIE
			mpie := (mstatus >> 7) & 0x1
			mstatus = (mstatus & ^uint64(1<<3)) | (mpie << 3)

			// 3. Set MPIE to 1 and MPP to 0 (Standard RISC-V behavior)
			mstatus |= (1 << 7)
			mstatus &= ^uint64(0x1800)

			if cpu.Trace {
				cpu.funcName(cpu.PC)
				cpu.tracef(raw, instr, "mepc=%x, mode=%v",
					cpu.GetCSR(CsrMepc), cpu.Mode())
			}

			// 4. Finalize Jump
			cpu.SetCSR(CsrMstatus, mstatus)
			cpu.PC = cpu.GetCSR(CsrMepc)

			continue

		case isa.Ecall:
			var cause uint64
			var target isa.PrivilegeMode
			switch cpu.Mode() {
			case isa.ModeU:
				cause = isa.CauseEcallU
				target = isa.ModeS
			case isa.ModeS:
				cause = isa.CauseEcallS
				target = isa.ModeM
			case isa.ModeM:
				cause = isa.CauseEcallM
				target = isa.ModeM
			default:
				return isa.NewTrap(0, cpu.PC, isa.CauseIllegalInstr, 0,
					fmt.Errorf("ecall in %v-mode", cpu.Mode()))
			}
			if cpu.Trace {
				cpu.funcName(cpu.PC)
				cpu.tracef(raw, instr,
					"mode=%v, target=%v, cause=%v, a7=%x, a6=%x, a0=%x, a1=%x",
					cpu.Mode(), target, cause, cpu.X[isa.A7], cpu.X[isa.A6],
					cpu.X[isa.A0], cpu.X[isa.A1])
			}

			return cpu.Trap(target, cause, 0, nil)

		case isa.Fld:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v, err := cpu.MMU.Load64(addr)
			if err != nil {
				return err
			}
			cpu.F[instr.Rd] = math.Float64frombits(v)

		case isa.Flw:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v32, err := cpu.MMU.Load32(addr)
			if err != nil {
				return err
			}
			v64 := uint64(v32)
			v64 |= uint64(0xffffffff) << 32
			cpu.F[instr.Rd] = math.Float64frombits(v64)

		case isa.Fsd:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v := math.Float64bits(cpu.F[instr.Rs2])
			if err := cpu.MMU.Store64(addr, v); err != nil {
				return err
			}

		case isa.Fsw:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v := math.Float32bits(float32(cpu.F[instr.Rs2]))
			if err := cpu.MMU.Store32(addr, uint64(v)); err != nil {
				return err
			}

		case isa.Fence:
			// XXX fence

		case isa.SfenceVMA:
			cpu.MMU.FlushTLB()

		case isa.FeqS:
			b1 := math.Float64bits(cpu.F[instr.Rs1])
			b2 := math.Float64bits(cpu.F[instr.Rs2])

			if math.Float32frombits(uint32(b1)) ==
				math.Float32frombits(uint32(b2)) {
				cpu.X[instr.Rd] = 1
			} else {
				cpu.X[instr.Rd] = 0
			}

		case isa.Jal:
			cpu.X[instr.Rd] = cpu.PC + uint64(size)
			cpu.PC = uint64(int64(cpu.PC) + int64(instr.Imm))
			continue

		case isa.Jalr:
			t := cpu.PC + uint64(size)
			cpu.PC = uint64(int64(cpu.X[instr.Rs1])+int64(instr.Imm)) &^ 1
			cpu.X[instr.Rd] = t
			continue

		case isa.Lbu:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v, err := cpu.MMU.Load8(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(v)

		case isa.Lb:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v, err := cpu.MMU.Load8(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int8(v)))

		case isa.Ld:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))

			// Direct TLB check.
			vpn := addr >> 12
			tlb := &cpu.MMU.TLB[vpn&0xfff]

			if tlb.VPN == vpn && tlb.Flags.Readable() {
				// Fast path: TLB hit.
				paddr := tlb.Page | (addr & uint64(tlb.OffsetMask))
				cpu.X[instr.Rd] =
					bo.Uint64(cpu.MMU.Mem.RAM[cpu.MMU.Mem.Offset(paddr):])
			} else {
				// Slow path fallback.
				v, err := cpu.MMU.Load64(addr)
				if err != nil {
					return err
				}
				cpu.X[instr.Rd] = v
			}

		case isa.Lh:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v, err := cpu.MMU.Load16(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int16(v)))

		case isa.Lhu:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v, err := cpu.MMU.Load16(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(v)

		case isa.Lui:
			cpu.X[instr.Rd] = uint64(instr.Imm)

		case isa.Lw:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v, err := cpu.MMU.Load32(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int32(v)))

		case isa.Lwu:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v, err := cpu.MMU.Load32(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(v)

		case isa.Mul:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] * cpu.X[instr.Rs2]

		case isa.Mulh:
			// XXX check this or use big.Int
			a := int64(cpu.X[instr.Rs1])
			b := int64(cpu.X[instr.Rs2])
			h, _ := bits.Mul64(uint64(a), uint64(b))
			hi := int64(h)

			// Correct the sign.
			if a < 0 {
				hi -= b
			}
			if b < 0 {
				hi -= a
			}
			cpu.X[instr.Rd] = uint64(hi)

		case isa.Mulhu:
			hi, _ := bits.Mul64(cpu.X[instr.Rs1], cpu.X[instr.Rs2])
			cpu.X[instr.Rd] = hi

		case isa.Mulw:
			cpu.X[instr.Rd] = uint64(int64(int32(cpu.X[instr.Rs1] *
				cpu.X[instr.Rs2])))

		case isa.Or:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] | cpu.X[instr.Rs2]

		case isa.Ori:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] | uint64(int64(instr.Imm))

		case isa.Rem:
			if cpu.X[instr.Rs2] == 0 {
				cpu.X[instr.Rd] = cpu.X[instr.Rs1]
			} else {
				cpu.X[instr.Rd] = uint64(int64(cpu.X[instr.Rs1]) %
					int64(cpu.X[instr.Rs2]))
			}

		case isa.Remu:
			if cpu.X[instr.Rs2] == 0 {
				cpu.X[instr.Rd] = cpu.X[instr.Rs1]
			} else {
				cpu.X[instr.Rd] = cpu.X[instr.Rs1] % cpu.X[instr.Rs2]
			}

		case isa.Remuw:
			if cpu.X[instr.Rs2] == 0 {
				cpu.X[instr.Rd] = uint64(uint32(cpu.X[instr.Rs1]))
			} else {
				cpu.X[instr.Rd] = uint64(uint32(cpu.X[instr.Rs1]) %
					uint32(cpu.X[instr.Rs2]))
			}

		case isa.Remw:
			if cpu.X[instr.Rs2] == 0 {
				cpu.X[instr.Rd] = uint64(int64(int32(cpu.X[instr.Rs1])))
			} else {
				cpu.X[instr.Rd] = uint64(int64(int32(cpu.X[instr.Rs1]) %
					int32(cpu.X[instr.Rs2])))
			}

		case isa.Sb:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			if err := cpu.MMU.Store8(addr, cpu.X[instr.Rs2]); err != nil {
				return err
			}
			cpu.HasReservation = false

		case isa.Sd:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))

			// Direct TLB check.
			vpn := addr >> 12
			tlb := &cpu.MMU.TLB[vpn&0xfff]

			if tlb.VPN == vpn && tlb.Flags.Writable() {
				// Fast path: TLB hit.
				paddr := tlb.Page | (addr & uint64(tlb.OffsetMask))
				bo.PutUint64(cpu.MMU.Mem.RAM[cpu.MMU.Mem.Offset(paddr):],
					cpu.X[instr.Rs2])
			} else {
				// Slow path fallback.
				if err := cpu.MMU.Store64(addr, cpu.X[instr.Rs2]); err != nil {
					return err
				}
			}
			cpu.HasReservation = false

		case isa.Sh:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			if err := cpu.MMU.Store16(addr, cpu.X[instr.Rs2]); err != nil {
				return err
			}
			cpu.HasReservation = false

		case isa.Sw:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			if err := cpu.MMU.Store32(addr, cpu.X[instr.Rs2]); err != nil {
				return err
			}
			cpu.HasReservation = false

		case isa.Sll:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] << (cpu.X[instr.Rs2] & 0b111111)

		case isa.Sllw:
			cpu.X[instr.Rd] = uint64(int64(int32(cpu.X[instr.Rs1]) <<
				(cpu.X[instr.Rs2] & 0b11111)))

		case isa.Slli:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] << instr.Imm

		case isa.Slliw:
			cpu.X[instr.Rd] = uint64(int64(int32(cpu.X[instr.Rs1]) <<
				instr.Imm))

		case isa.Slt:
			if int64(cpu.X[instr.Rs1]) < int64(cpu.X[instr.Rs2]) {
				cpu.X[instr.Rd] = 1
			} else {
				cpu.X[instr.Rd] = 0
			}

		case isa.Slti:
			if int64(cpu.X[instr.Rs1]) < int64(instr.Imm) {
				cpu.X[instr.Rd] = 1
			} else {
				cpu.X[instr.Rd] = 0
			}

		case isa.Sltiu:
			if cpu.X[instr.Rs1] < uint64(instr.Imm) {
				cpu.X[instr.Rd] = 1
			} else {
				cpu.X[instr.Rd] = 0
			}

		case isa.Sltu:
			if cpu.X[instr.Rs1] < cpu.X[instr.Rs2] {
				cpu.X[instr.Rd] = 1
			} else {
				cpu.X[instr.Rd] = 0
			}

		case isa.Srai:
			cpu.X[instr.Rd] = uint64(int64(cpu.X[instr.Rs1]) >> instr.Imm)

		case isa.Sraiw:
			cpu.X[instr.Rd] = uint64(int64(int32(cpu.X[instr.Rs1]) >>
				instr.Imm))

		case isa.Sraw:
			cpu.X[instr.Rd] = uint64(int64(int32(cpu.X[instr.Rs1]) >>
				(cpu.X[instr.Rs2] & 0b11111)))

		case isa.Srl:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] >> (cpu.X[instr.Rs2] & 0b111111)

		case isa.Srlw:
			cpu.X[instr.Rd] = uint64(uint32(cpu.X[instr.Rs1]) >>
				(cpu.X[instr.Rs2] & 0b11111))

		case isa.Srli:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] >> instr.Imm

		case isa.Srliw:
			cpu.X[instr.Rd] = uint64(uint32(cpu.X[instr.Rs1]) >> instr.Imm)

		case isa.Sub:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] - cpu.X[instr.Rs2]

		case isa.Subw:
			cpu.X[instr.Rd] = uint64(int64(int32(cpu.X[instr.Rs1] -
				cpu.X[instr.Rs2])))

		case isa.Xor:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] ^ cpu.X[instr.Rs2]

		case isa.Xori:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] ^ uint64(int64(instr.Imm))

			// Control and Status Registers (CSRs).
		case isa.Csrrs:
			csr := CSR(instr.Imm)
			t := cpu.GetCSR(csr)
			if instr.Rs1 != isa.Zero {
				cpu.SetCSRX(csr, t|cpu.X[instr.Rs1], raw, instr)
			}
			cpu.X[instr.Rd] = t

		case isa.Csrrc:
			csr := CSR(instr.Imm)
			t := cpu.GetCSR(csr)
			if instr.Rs1 != isa.Zero {
				cpu.SetCSRX(csr, t & ^cpu.X[instr.Rs1], raw, instr)
			}
			cpu.X[instr.Rd] = t

		case isa.Csrrci:
			csr := CSR(instr.Imm)
			t := cpu.GetCSR(csr)
			if instr.Rs1 != isa.Zero {
				cpu.SetCSRX(csr, t & ^uint64(instr.Rs1), raw, instr)
			}
			cpu.X[instr.Rd] = t

		case isa.Csrrsi:
			csr := CSR(instr.Imm)
			t := cpu.GetCSR(csr)
			cpu.SetCSRX(csr, t|uint64(instr.Rs1), raw, instr)
			cpu.X[instr.Rd] = t

		case isa.Csrrw:
			csr := CSR(instr.Imm)
			oldCSR := cpu.GetCSR(csr)    // 1. Capture old CSR value
			valToSet := cpu.X[instr.Rs1] // 2. Capture value from GPR

			cpu.SetCSRX(csr, valToSet, raw, instr) // 3. Update CSR
			cpu.X[instr.Rd] = oldCSR               // 4. Update GPR with old CSR

		case isa.Csrrwi:
			csr := CSR(instr.Imm)
			cpu.SetCSRX(csr, uint64(instr.Rs1), raw, instr)

			// Atomic (A extension).

		case isa.AmoswapD:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.MMU.Load64(addr)
			if err != nil {
				return err
			}
			err = cpu.MMU.Store64(addr, cpu.X[instr.Rs2])
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = v

		case isa.AmoswapW:
			addr := cpu.X[instr.Rs1]

			if addr%4 != 0 {
				return isa.NewTrap(0, cpu.PC, isa.CauseStoreAddrMisaligned,
					addr, nil)
			}

			v, err := cpu.MMU.Load32(addr)
			if err != nil {
				return err
			}
			err = cpu.MMU.Store32(addr, cpu.X[instr.Rs2])
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int32(v)))

		case isa.AmoaddD:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.MMU.Load64(addr)
			if err != nil {
				return err
			}
			t := v + uint64(cpu.X[instr.Rs2])
			err = cpu.MMU.Store64(addr, t)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = v

		case isa.AmoaddW:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.MMU.Load32(addr)
			if err != nil {
				return err
			}
			t := uint64(int64(int32(v) + int32(cpu.X[instr.Rs2])))
			err = cpu.MMU.Store32(addr, t)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int32(v)))

		case isa.AmoandD:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.MMU.Load64(addr)
			if err != nil {
				return err
			}
			t := v & cpu.X[instr.Rs2]
			err = cpu.MMU.Store64(addr, t)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = v

		case isa.AmoandW:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.MMU.Load32(addr)
			if err != nil {
				return err
			}
			t := uint64(int64(int32(v) & int32(cpu.X[instr.Rs2])))
			err = cpu.MMU.Store32(addr, t)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int32(v)))

		case isa.AmoorD:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.MMU.Load64(addr)
			if err != nil {
				return err
			}
			t := v | cpu.X[instr.Rs2]
			err = cpu.MMU.Store64(addr, t)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = v

		case isa.AmoorW:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.MMU.Load32(addr)
			if err != nil {
				return err
			}
			t := uint64(int64(int32(v) | int32(cpu.X[instr.Rs2])))
			err = cpu.MMU.Store32(addr, t)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int32(v)))

		case isa.LrW:
			addr := cpu.X[instr.Rs1]
			// Optional: Check alignment (4-byte)
			if addr%4 != 0 {
				return isa.NewTrap(0, cpu.PC, isa.CauseLoadAddrMisaligned, addr, nil)
			}
			v, err := cpu.MMU.Load32(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int32(v)))

			// Register the reservation
			cpu.Reservation = addr
			cpu.HasReservation = true

		case isa.LrD:
			addr := cpu.X[instr.Rs1]
			// Optional: Check alignment (8-byte)
			if addr%8 != 0 {
				return isa.NewTrap(0, cpu.PC, isa.CauseLoadAddrMisaligned, addr, nil)
			}
			v, err := cpu.MMU.Load64(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = v

			// Register the reservation
			cpu.Reservation = addr
			cpu.HasReservation = true

		case isa.ScW:
			addr := cpu.X[instr.Rs1]
			// Check alignment
			if addr%4 != 0 {
				cpu.HasReservation = false
				return isa.NewTrap(0, cpu.PC, isa.CauseStoreAddrMisaligned, addr, nil)
			}

			// SC succeeds only if the reservation matches
			if cpu.HasReservation && cpu.Reservation == addr {
				err := cpu.MMU.Store32(addr, cpu.X[instr.Rs2])
				if err != nil {
					cpu.HasReservation = false
					return err
				}
				cpu.X[instr.Rd] = 0 // 0 = Success
			} else {
				cpu.X[instr.Rd] = 1 // 1 = Failure
			}
			cpu.HasReservation = false

		case isa.ScD:
			addr := cpu.X[instr.Rs1]
			if addr%8 != 0 {
				cpu.HasReservation = false
				return isa.NewTrap(0, cpu.PC, isa.CauseStoreAddrMisaligned, addr, nil)
			}

			if cpu.HasReservation && cpu.Reservation == addr {
				err := cpu.MMU.Store64(addr, cpu.X[instr.Rs2])
				if err != nil {
					cpu.HasReservation = false
					return err
				}
				cpu.X[instr.Rd] = 0
			} else {
				cpu.X[instr.Rd] = 1
			}
			cpu.HasReservation = false

			// Floating point extension.

		case isa.FaddD:
			cpu.F[instr.Rd] = cpu.F[instr.Rs1] + cpu.F[instr.Rs2]

		case isa.FsubD:
			cpu.F[instr.Rd] = cpu.F[instr.Rs1] - cpu.F[instr.Rs2]

		case isa.FmulD:
			cpu.F[instr.Rd] = cpu.F[instr.Rs1] * cpu.F[instr.Rs2]

		case isa.FeqD:
			if cpu.F[instr.Rs1] == cpu.F[instr.Rs2] {
				cpu.X[instr.Rd] = 1
			} else {
				cpu.X[instr.Rd] = 0
			}

		case isa.FmvDX:
			cpu.F[instr.Rd] = math.Float64frombits(cpu.X[instr.Rs1])

		case isa.FmvWX:
			v := uint64(uint32(cpu.X[instr.Rs1]))
			v |= uint64(0xffffffff) << 32
			cpu.F[instr.Rd] = math.Float64frombits(v)

		case isa.FmvXD:
			cpu.X[instr.Rd] = math.Float64bits(cpu.F[instr.Rs1])

		case isa.FmaddD:
			// Imm is Rs3
			cpu.F[instr.Rd] = cpu.F[instr.Rs1]*cpu.F[instr.Rs2] +
				cpu.F[instr.Imm]

		case isa.FcvtDL:
			// XXX The rounding mode (RM) is specified in the fcsr
			// (Floating-point Control and Status Register)
			cpu.F[instr.Rd] = float64(int64(cpu.X[instr.Rs1]))

		case isa.FcvtDLU:
			// XXX The rounding mode (RM) is specified in the fcsr
			// (Floating-point Control and Status Register)
			cpu.F[instr.Rd] = float64(cpu.X[instr.Rs1])

		case isa.FcvtLD:
			// XXX If the value is out of range, fcsr.fflags.NV is set
			// to 1
			cpu.X[instr.Rd] = uint64(int64(cpu.F[instr.Rs1]))

		case isa.FcvtWD:
			cpu.X[instr.Rd] = uint64(int64(int32(cpu.F[instr.Rs1])))

		case isa.FcvtWUD:
			cpu.X[instr.Rd] = uint64(uint32(cpu.F[instr.Rs1]))

		default:
			return isa.NewTrap(0, cpu.PC, isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("instruction %v[0x%x] not implemented yet",
					instr, raw))
		}
		cpu.PC += uint64(size)
	}
}

func (cpu *CPU) funcName(pc uint64) *SymEntry {
	if cpu.Symtab == nil {
		return nil
	}
	// OpenSBI range: no kernel symbols here
	if pc >= 0x80000000 && pc < 0x80200000 {
		return nil
	}
	mapped := pc
	if pc > 0x80200000 && pc < 0x100000000 {
		// Physical address during early boot (MMU off).
		// Kernel is loaded at 0x80200000 physical = 0xffffffff80000000 virtual.
		// delta = 0xffffffff80000000 - 0x80200000 = 0xffffffff7fe00000
		mapped = pc - 0x200000 + 0xffffffff00000000
		fmt.Printf(" - mapped=%x\n", mapped)
	}
	entry := cpu.Symtab.Resolve(mapped)
	if entry == nil {
		return nil
	}
	fmt.Printf("%8x:  %s+0x%x\n", pc, entry.Name, mapped-entry.Addr)
	return entry
}

func (cpu *CPU) tracef(raw uint32, instr isa.Instr,
	format string, args ...interface{}) {

	cpu.trace(raw, instr, fmt.Sprintf(format, args...))
}

func (cpu *CPU) trace(raw uint32, instr isa.Instr, msg string) {
	var line string
	if raw&0b11 == 0b11 {
		line = fmt.Sprintf("%8x:  %08x   %v", cpu.PC, raw, instr)
	} else {
		line = fmt.Sprintf("%8x:  %04x       %v", cpu.PC, raw, instr)
	}
	if len(msg) == 0 {
		op, ok := isa.Operands[instr.Op]
		if ok && len(op.Desc) > 0 && instr.Op != cpu.lastDescOp {
			cpu.lastDescOp = instr.Op
			msg = op.Desc
		}
	}
	if len(msg) > 0 {
		for len(line) < 46 {
			line += " "
		}
		line += fmt.Sprintf(" # %s", msg)
	}
	fmt.Println(line)

}
