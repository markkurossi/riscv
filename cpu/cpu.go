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

	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/memory"
	"github.com/markkurossi/riscv/mmu"
)

var (
	bo = binary.LittleEndian
)

type Mode uint8

const (
	ModeU Mode = iota
	ModeS
	ModeH
	ModeM
)

var modes = map[Mode]string{
	ModeU: "U",
	ModeS: "S",
	ModeH: "H",
	ModeM: "M",
}

func (m Mode) String() string {
	name, ok := modes[m]
	if ok {
		return name
	}
	return fmt.Sprintf("{Mode %d}", int(m))
}

type TrapHandler func(cpu *CPU, trap *isa.Trap) (bool, error)

type CPU struct {
	PID uint64 // XXX how is this set? This is an usermode emulator field.

	Trace bool

	X [32]uint64
	F [32]float64

	Mode Mode

	CSR [4096]uint64

	PC uint64

	decodeCache [4096]struct {
		Raw   uint32
		Instr isa.Instr
	}

	// Instruction count
	Instret uint64

	MMU *mmu.MMU

	TrapHandler TrapHandler

	lastDescOp isa.Op
}

func (cpu *CPU) Run() error {
	for {
		err := cpu.loop()
		if err != nil {
			if trap, ok := errors.AsType[*isa.Trap](err); ok {
				trap.PC = cpu.PC

				// XXX: rethink the CSR storing:
				// - create CPU.Trap() that saves things accordingly
				// - fix ecall/ebreak to change state as needed so
				// that CPU.Trap() does the right thing
				// - if we are here with trap.PC == nil, store state
				// so that it matches MMU's page faults - MMU does not
				// have PC so save it here.
				//
				// - Save Mode: The current mode (1 for S) is saved
				// into mstatus.MPP (Machine Previous Privilege).
				//
				// - Save Interrupt State: The current mstatus.MIE
				// (Interrupt Enable) is saved into mstatus.MPIE, and
				// then MIE is set to 0 (disabling interrupts).

				switch cpu.Mode {
				case ModeS, ModeU:
					cpu.StoreCSR(CsrSepc, trap.PC)
					cpu.StoreCSR(CsrScause, trap.Cause)
					cpu.StoreCSR(CsrStvec, trap.Tval)

				case ModeM:
					cpu.StoreCSR(CsrMepc, trap.PC)
					cpu.StoreCSR(CsrMcause, trap.Cause)
					cpu.StoreCSR(CsrMtvec, trap.Tval)

				default:
					return fmt.Errorf("unhandled %v mode trap %w",
						cpu.Mode, err)
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
		return isa.NewTrap(cpu.PC, isa.CauseInstAddrMisaligned, cpu.PC, nil)
	}

	var codePagenum uint64
	var codePage []byte

	for {
		var instr isa.Instr
		var err error
		var size int

		cpu.X[isa.Zero] = 0

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
			return isa.NewTrap(cpu.PC, isa.CauseIllegalInstr, uint64(raw), err)
		}
		cpu.Instret++

		if false {
			cpu.trace(raw, instr, "")
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
			// XXX mie, mpie, mpp
			var tval uint64
			switch cpu.Mode {
			case ModeS:
				tval = cpu.LoadCSR(CsrStvec)
			case ModeM:
				tval = cpu.LoadCSR(CsrMtvec)
			}

			if cpu.Trace {
				cpu.tracef(raw, instr, "tval=%x", tval)
			}

			return isa.NewTrap(cpu.PC, isa.CauseBreakpoint, tval, nil)

		case isa.Sret:
			// The sret semantics:
			//
			// 	 PC           ← sepc
			// 	 privilege    ← sstatus.SPP
			// 	 sstatus.SIE  ← sstatus.SPIE
			// 	 sstatus.SPIE ← 1
			// 	 sstatus.SPP  ← U (0)

			if cpu.Trace {
				cpu.tracef(raw, instr, "sepc=%x", cpu.LoadCSR(CsrSepc))
			}
			cpu.PC = cpu.LoadCSR(CsrSepc)

		case isa.Mret:
			// The mret semantics:
			//
			// 	 PC           ← mepc
			// 	 privilege    ← mstatus.MPP
			// 	 mstatus.MIE  ← mstatus.MPIE
			// 	 mstatus.MPIE ← 1
			// 	 mstatus.MPP  ← U (0)

			if cpu.Trace {
				cpu.tracef(raw, instr, "mepc=%x", cpu.LoadCSR(CsrMepc))
			}
			cpu.Mode = ModeS

			cpu.PC = cpu.LoadCSR(CsrMepc)
			if cpu.PC == 0 && false {
				cpu.PC = 0x8021cbd0 + 4
			}
			continue

		case isa.Ecall:
			var cause uint64
			switch cpu.Mode {
			case ModeU:
				cause = isa.CauseEcallU
			case ModeS:
				cause = isa.CauseEcallS
			case ModeM:
				cause = isa.CauseEcallM
			default:
				return isa.NewTrap(cpu.PC, isa.CauseIllegalInstr, 0,
					fmt.Errorf("ecall in %v-mode", cpu.Mode))
			}
			return isa.NewTrap(cpu.PC, cause, 0, nil)

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

		case isa.SfenceVMA:
			// XXX Clear mmu.TLB

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

		case isa.Sh:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			if err := cpu.MMU.Store16(addr, cpu.X[instr.Rs2]); err != nil {
				return err
			}

		case isa.Sw:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			if err := cpu.MMU.Store32(addr, cpu.X[instr.Rs2]); err != nil {
				return err
			}

		case isa.Xor:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] ^ cpu.X[instr.Rs2]

		case isa.Xori:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] ^ uint64(int64(instr.Imm))

			// Control and Status Registers (CSRs).
		case isa.Csrrs:
			csr := CSR(instr.Imm)
			t := cpu.LoadCSR(csr)
			if instr.Rs1 != isa.Zero {
				cpu.StoreCSR(csr, t|cpu.X[instr.Rs1])
			}
			cpu.X[instr.Rd] = t

		case isa.Csrrc:
			csr := CSR(instr.Imm)
			t := cpu.LoadCSR(csr)
			if instr.Rs1 != isa.Zero {
				cpu.StoreCSR(csr, t & ^cpu.X[instr.Rs1])
			}
			cpu.X[instr.Rd] = t

		case isa.Csrrsi:
			csr := CSR(instr.Imm)
			t := cpu.LoadCSR(csr)
			cpu.StoreCSR(csr, t|uint64(instr.Rs1))
			cpu.X[instr.Rd] = t

		case isa.Csrrw:
			csr := CSR(instr.Imm)
			t := cpu.LoadCSR(csr)
			cpu.StoreCSR(csr, cpu.X[instr.Rs1])
			cpu.X[instr.Rd] = t

		case isa.Csrrwi:
			csr := CSR(instr.Imm)
			cpu.StoreCSR(csr, uint64(instr.Rs1))

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
				return isa.NewTrap(cpu.PC, isa.CauseStoreAddrMisaligned, addr,
					nil)
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

		case isa.LrD:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.MMU.Load64(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = v

		case isa.LrW:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.MMU.Load32(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int32(v)))

		case isa.ScD:
			addr := cpu.X[instr.Rs1]
			err := cpu.MMU.Store64(addr, cpu.X[instr.Rs2])
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = 0

		case isa.ScW:
			addr := cpu.X[instr.Rs1]
			err := cpu.MMU.Store32(addr, cpu.X[instr.Rs2])
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = 0

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
			return isa.NewTrap(cpu.PC, isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("instruction %v[0x%x] not implemented yet",
					instr, raw))
		}
		cpu.PC += uint64(size)
	}
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
		for len(line) < 47 {
			line += " "
		}
		line += fmt.Sprintf("# %s", msg)
	}
	fmt.Println(line)

}
