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

type Syscall func(cpu *CPU, id, a0, a1, a2, a3, a4, a5 uint64) (
	uint64, error)

type TrapHandler func(cpu *CPU, trap *isa.Trap) (bool, error)

type CSR struct {
	Status       uint64
	Tvec         uint64
	Scratch      uint64
	Epc          uint64 // Exception PC.
	Cause        uint64 // Exception cause.
	Tval         uint64 // Trap value e.g. virtual address causing page fault.
	Ip           uint64
	Ie           uint64
	Pmpcfg       [8]uint64
	Pmpaddr      [16]uint64
	Counteren    uint64
	Countinhibit uint64
	Envcfg       uint64
	Stateen      [4]uint64
	Ideleg       uint64
	Edeleg       uint64
	Tselect      uint64
	Tdata1       uint64
}

type CPU struct {
	PID uint64 // XXX how is this set?
	X   [32]uint64
	F   [32]float64

	// S-Mode CSRs (Direct Access)
	S CSR
	M CSR

	PC uint64

	decodeCache [4096]struct {
		Raw   uint32
		Instr isa.Instr
	}

	// Instruction count
	Instret uint64

	MMU *mmu.MMU

	Syscall     Syscall
	TrapHandler TrapHandler
}

func (cpu *CPU) Run() error {
	for {
		err := cpu.loop()
		if err != nil {
			if trap, ok := errors.AsType[*isa.Trap](err); ok {
				trap.PC = cpu.PC

				// XXX check mode.
				cpu.M.Epc = trap.PC
				cpu.M.Tval = trap.Tval
				cpu.M.Cause = trap.Cause
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
	var lastDescOp isa.Op

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
			var line string
			if size == 4 {
				line = fmt.Sprintf("%8x:  %08x   %v", cpu.PC, raw, instr)
			} else {
				line = fmt.Sprintf("%8x:  %04x       %v", cpu.PC, raw, instr)
			}
			op, ok := isa.Operands[instr.Op]
			if ok && len(op.Desc) > 0 && instr.Op != lastDescOp {
				lastDescOp = instr.Op

				for len(line) < 47 {
					line += " "
				}
				line += fmt.Sprintf("# %s", op.Desc)
			}
			fmt.Println(line)
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
			return isa.NewTrap(cpu.PC, isa.CauseBreakpoint, cpu.PC,
				fmt.Errorf("Mtvec=%x, Stvec=%x", cpu.M.Tvec, cpu.S.Tvec))

		case isa.Mret:
			// XXX mie, mpie, mpp

			// Using mepc directly as the return PC without checking
			// it was correctly written — confirm mepc = 0x80012bd4
			// (the ebreak address, not +4) going in, and that the
			// handler's csrw mepc, a5 (with a5 = 0x80012bd8) updated
			// it before mret reads it.

			// Not updating mstatus fields — MPP must be cleared to 0
			// (U-mode) and MPIE set to 1 after mret, even if the
			// return mode is M-mode. Failing to clear MPP can corrupt
			// later trap handling.

			if false {
				fmt.Printf("%v: M.Epc=%x\n", instr, cpu.M.Epc)
			}

			cpu.PC = cpu.M.Epc
			continue

		case isa.Ecall:
			if true {
				// XXX Track CPU mode
				return isa.NewTrap(cpu.PC, isa.CauseEcallS, 0, nil)
			}
			v, err := cpu.Syscall(cpu, cpu.X[isa.A7],
				cpu.X[isa.A0], cpu.X[isa.A1], cpu.X[isa.A2],
				cpu.X[isa.A3], cpu.X[isa.A4], cpu.X[isa.A5])
			if err != nil {
				return err
			}
			cpu.X[isa.A0] = v

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
			switch instr.Imm {
			case 0x14d: // scontext
				cpu.X[instr.Rd] = 0

			case 0x300:
				t := cpu.M.Status
				if instr.Rs1 != isa.Zero {
					// Standard bit-setting logic
					cpu.M.Status = t | cpu.X[instr.Rs1]
				}
				cpu.X[instr.Rd] = t

			case 0x301: // misa - Machine Instruction Set Architecture
				// RV64IMAFDC
				var misaValue uint64 = (2 << 62) | // MXLEN = 64
					(1 << 0) | // A (Atomic)
					(1 << 2) | // C (Compressed)
					(1 << 3) | // D (Double)
					(1 << 5) | // F (Float)
					(1 << 8) | // I (Integer)
					(1 << 12) | // M (Multiply)
					(1 << 18) | // S (Supervisor)
					(1 << 20) // U (User mode)
				cpu.X[instr.Rd] = misaValue

			case 0x302:
				cpu.X[instr.Rd] = cpu.M.Edeleg

			case 0x303:
				cpu.X[instr.Rd] = cpu.M.Ideleg

			case 0x306: // Mcounteren
				t := cpu.M.Counteren
				if instr.Rs1 != isa.Zero {
					// Standard bit-setting logic
					cpu.M.Counteren = t | cpu.X[instr.Rs1]
				}
				cpu.X[instr.Rd] = t

			case 0x30a: // Menvcfg
				t := cpu.M.Envcfg
				if instr.Rs1 != isa.Zero {
					cpu.M.Envcfg = t | cpu.X[instr.Rs1]
				}
				cpu.X[instr.Rd] = t

			case 0x30c: // mstateen0
				cpu.X[instr.Rd] = 0

			case 0x320: // Mcountinhibit
				t := cpu.M.Countinhibit
				if instr.Rs1 != isa.Zero {
					// Update the inhibit mask
					cpu.M.Countinhibit = t | cpu.X[instr.Rs1]
				}
				cpu.X[instr.Rd] = t

			case 0x321: // mhpmevent3
				cpu.X[instr.Rd] = 0

			case 0x340: // Mscratch
				t := cpu.M.Scratch
				if instr.Rs1 != isa.Zero {
					cpu.M.Scratch = t | cpu.X[instr.Rs1]
				}
				cpu.X[instr.Rd] = t

			case 0x341: // Mepc
				t := cpu.M.Epc
				if instr.Rs1 != isa.Zero {
					cpu.M.Epc = t | cpu.X[instr.Rs1]
				}
				cpu.X[instr.Rd] = t

			case 0x3a0: // Pmpcfg0
				t := cpu.M.Pmpcfg[0]
				if instr.Rs1 != isa.Zero {
					cpu.M.Pmpcfg[0] = t | cpu.X[instr.Rs1]
				}
				cpu.X[instr.Rd] = t

			case 0x3b0: // Pmpcfg2
				t := cpu.M.Pmpcfg[2]
				if instr.Rs1 != isa.Zero {
					cpu.M.Pmpcfg[2] = t | cpu.X[instr.Rs1]
				}
				cpu.X[instr.Rd] = t

			case 0x3b1: // Pmpaddr8
				t := cpu.M.Pmpaddr[8]
				if instr.Rs1 != isa.Zero {
					cpu.M.Pmpaddr[8] = t | cpu.X[instr.Rs1]
				}
				cpu.X[instr.Rd] = t

			case 0x7a0, 0x7a1, 0x7a2, 0x7a3, 0x7a4: // Debug Triggers
				// Return 0 to tell OpenSBI "I have 0 hardware triggers"
				cpu.X[instr.Rd] = 0

			case 0xc01: // time - RDCYCLE instruction
				cpu.X[instr.Rd] = cpu.Instret

			case 0xda0: // Senvcfg
				t := cpu.S.Envcfg // Or however you store S-mode state
				if instr.Rs1 != isa.Zero {
					cpu.S.Envcfg = t | cpu.X[instr.Rs1]
				}
				cpu.X[instr.Rd] = t

			case 0xf11: // mvendorid
				cpu.X[instr.Rd] = 0

			case 0xf12: // marchid
				cpu.X[instr.Rd] = 0 // Architecture ID

			case 0xf13: // mimpid
				cpu.X[instr.Rd] = 0 // Implementation ID

			case 0xf14: // mhartid
				cpu.X[instr.Rd] = 0 // Hardware Thread ID

			case 0xfb0: // scountinhibit
				cpu.X[instr.Rd] = 0

			default:
				if instr.Imm >= 0xb03 && instr.Imm <= 0xb1f {
					// Mhpmcounters
					cpu.X[instr.Rd] = 0 // Counters return 0
				} else {
					return isa.NewTrap(cpu.PC, isa.CauseIllegalInstr,
						uint64(raw), fmt.Errorf("%v", instr))
				}
			}

		case isa.Csrrc:
			switch instr.Imm {
			case 0x100:
				t := cpu.S.Status
				if instr.Rs1 != isa.Zero {
					cpu.S.Status = t & ^cpu.X[instr.Rs1]
				}
				cpu.X[instr.Rd] = t

			case 0x300:
				t := cpu.M.Status
				if instr.Rs1 != isa.Zero {
					cpu.M.Status = t & ^cpu.X[instr.Rs1]
				}
				cpu.X[instr.Rd] = t
			default:
				return isa.NewTrap(cpu.PC, isa.CauseIllegalInstr, uint64(raw),
					fmt.Errorf("%v", instr))
			}

		case isa.Csrrsi:
			switch instr.Imm {
			case 0x304:
				t := cpu.M.Ie
				cpu.M.Ie = t | uint64(instr.Imm)
				cpu.X[instr.Rd] = t

			default:
				return isa.NewTrap(cpu.PC, isa.CauseIllegalInstr,
					uint64(raw), fmt.Errorf("%v", instr))
			}

		case isa.Csrrw:
			switch instr.Imm {
			case 0x104:
				t := cpu.S.Ie
				cpu.S.Ie = cpu.X[instr.Rs1]
				cpu.X[instr.Rd] = t

			case 0x105:
				t := cpu.S.Tvec
				cpu.S.Tvec = cpu.X[instr.Rs1]
				cpu.X[instr.Rd] = t

			case 0x106:
				t := cpu.S.Counteren
				cpu.S.Counteren = cpu.X[instr.Rs1]
				cpu.X[instr.Rd] = t

			case 0x144:
				t := cpu.S.Ip
				cpu.S.Ip = cpu.X[instr.Rs1]
				cpu.X[instr.Rd] = t

			case 0x300:
				t := cpu.M.Status
				cpu.M.Status = cpu.X[instr.Rs1]
				cpu.X[instr.Rd] = t

			case 0x302:
				t := cpu.M.Edeleg
				cpu.M.Edeleg = cpu.X[instr.Rs1]
				cpu.X[instr.Rd] = t

				// func (cpu *CPU) trap(exceptionCode uint64) {
				//     // If we are in S or U mode, and the bit is set in medeleg...
				//     if cpu.Mode <= ModeS && (cpu.M.edeleg & (1 << exceptionCode)) != 0 {
				//         // Jump to stvec (Supervisor Trap Vector)
				//         cpu.transferToSMode(exceptionCode)
				//     } else {
				//         // Jump to mtvec (Machine Trap Vector)
				//         cpu.transferToMMode(exceptionCode)
				//     }
				// }

			case 0x303:
				t := cpu.M.Ideleg
				cpu.M.Ideleg = cpu.X[instr.Rs1]
				cpu.X[instr.Rd] = t

				// // Simplified interrupt logic
				// if cpu.Mode < ModeM && (cpu.M.ideleg & (1 << interruptID)) != 0 {
				//     // Jump to stvec (Supervisor Trap Vector)
				//     cpu.TrapToSMode(interruptID)
				// } else {
				//     // Jump to mtvec (Machine Trap Vector)
				//     cpu.TrapToMMode(interruptID)
				// }

			case 0x304:
				t := cpu.M.Ie
				cpu.M.Ie = cpu.X[instr.Rs1]
				cpu.X[instr.Rd] = t

			case 0x305:
				t := cpu.M.Tvec
				cpu.M.Tvec = cpu.X[instr.Rs1]
				cpu.X[instr.Rd] = t

			case 0x306: // mcounteren
				cpu.M.Counteren = cpu.X[instr.Rs1]

			case 0x30a: // menvcfg
				cpu.M.Envcfg = cpu.X[instr.Rs1]

			case 0x30c: // mstateen0
				// Store the enable mask
				cpu.M.Stateen[0] = cpu.X[instr.Rs1]

			case 0x320: // mcountinhibit
				cpu.M.Countinhibit = cpu.X[instr.Rs1]

			case 0x340:
				t := cpu.M.Scratch
				cpu.M.Scratch = cpu.X[instr.Rs1]
				cpu.X[instr.Rd] = t

			case 0x341:
				t := cpu.M.Epc
				cpu.M.Epc = cpu.X[instr.Rs1]
				cpu.X[instr.Rd] = t

			case 0x3a0: // pmpcfg0
				cpu.M.Pmpcfg[0] = cpu.X[instr.Rs1]
			case 0x3b0: // pmpcfg2
				cpu.M.Pmpcfg[2] = cpu.X[instr.Rs1]

			case 0x3b1: // pmpaddr8
				// Store the boundary address
				cpu.M.Pmpaddr[8] = cpu.X[instr.Rs1]

			case 0x7a0: // tselect
				cpu.M.Tselect = cpu.X[instr.Rs1]
			case 0x7a1: // tdata1
				cpu.M.Tdata1 = cpu.X[instr.Rs1]

			case 0x7a4: // tinfo Returning 0 indicates no trigger
				// types are supported for the selected tselect.
				cpu.X[instr.Rd] = 0

			default:
				if instr.Imm >= 0xb03 && instr.Imm <= 0xb1f {
					// Mhpmcounters
				} else {
					return isa.NewTrap(cpu.PC, isa.CauseIllegalInstr,
						uint64(raw), fmt.Errorf("%v", instr))
				}
			}

		case isa.Csrrwi:
			switch instr.Imm {
			case 0x003: // fcsr
				// OpenSBI is writing 0. Store it so it can be read back.
				// XXX cpu.F.fcsr = uint32(instr.Imm)

			case 0x104: // S.Ie
				cpu.S.Ie = uint64(instr.Imm)

			case 0x106: // scounteren
				// OpenSBI is writing 7 here.
				cpu.S.Counteren = uint64(instr.Imm)

			case 0x140: // S.Scratch
				cpu.S.Scratch = uint64(instr.Imm)

			case 0x180: // satp
				// For now, OpenSBI is writing 0.
				// This just ensures translation remains 'Bare'.
				cpu.MMU.Satp = mmu.Satp(cpu.X[instr.Rs1])

			case 0x304:
				t := cpu.M.Ie
				cpu.M.Ie = uint64(instr.Rs1)
				cpu.X[instr.Rd] = t

			case 0x340:
				cpu.X[instr.Rd] = cpu.M.Scratch
				cpu.M.Scratch = uint64(instr.Rs1)

			case 0x344:
				t := cpu.M.Ip
				cpu.M.Ip = uint64(instr.Imm)
				cpu.X[instr.Rd] = t

			default:
				return isa.NewTrap(cpu.PC, isa.CauseIllegalInstr, uint64(raw),
					fmt.Errorf("%v", instr))
			}

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
