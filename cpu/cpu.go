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
	"time"

	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/memory"
)

var (
	bo = binary.LittleEndian
)

type Syscall func(cpu *CPU, id, a0, a1, a2, a3, a4, a5 uint64) (
	uint64, error)

type TrapHandler func(cpu *CPU, trap *Trap) (bool, error)

type CPU struct {
	PID uint64 // XXX how is this set?
	X   [32]uint64
	F   [32]float64

	// Address Translation.
	Satp Satp

	// Exception PC.
	Sepc uint64

	// Exception cause.
	Scause uint64

	// Trap Value - e.g. which virtual address caused page fault.
	Stval uint64

	PC uint64

	// Instruction count
	Instret uint64

	Memory memory.Memory
	TLB    [4096]TLBEntry

	Syscall     Syscall
	TrapHandler TrapHandler
}

func (cpu *CPU) Run() error {
	for {
		err := cpu.loop()
		if err != nil {
			if trap, ok := errors.AsType[*Trap](err); ok {
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
		return cpu.Trap(CauseInstAddrMisaligned, cpu.PC, nil)
	}

	for {
		var instrbuf [4]byte

		cpu.X[isa.Zero] = 0

		paddr, err := cpu.Map(cpu.PC, AccessExec)
		if err != nil {
			return err
		}
		err = cpu.Memory.Load(paddr, instrbuf[:2])
		if err != nil {
			return err
		}
		if instrbuf[0]&0b11 == 0b11 {
			// 32-bit instruction.
			if cpu.PC>>12 == (cpu.PC+2)>>12 {
				// Same page.
				paddr += 2
			} else {
				// 32-bit instruction crosses page boundary.
				paddr, err = cpu.Map(cpu.PC+2, AccessExec)
				if err != nil {
					return err
				}
			}
			err = cpu.Memory.Load(paddr, instrbuf[2:4])
			if err != nil {
				return err
			}
		}
		instr, size, err := isa.Decode(instrbuf[:])
		if err != nil {
			var raw uint64
			if size == 4 {
				raw = uint64(bo.Uint32(instrbuf[:]))
			} else {
				raw = uint64(bo.Uint16(instrbuf[:]))
			}
			return cpu.Trap(CauseIllegalInstr, raw, err)
		}
		cpu.Instret++

		if false {
			fmt.Printf("%8x:\t%08x\t%v\n", cpu.PC, instr.Raw, instr)
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

		case isa.Ecall:
			v, err := cpu.Syscall(cpu, cpu.X[isa.A7],
				cpu.X[isa.A0], cpu.X[isa.A1], cpu.X[isa.A2],
				cpu.X[isa.A3], cpu.X[isa.A4], cpu.X[isa.A5])
			if err != nil {
				return err
			}
			cpu.X[isa.A0] = v

		case isa.Fld:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v, err := cpu.UserUint64(addr)
			if err != nil {
				return err
			}
			cpu.F[instr.Rd] = math.Float64frombits(v)

		case isa.Flw:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v32, err := cpu.UserUint32(addr)
			if err != nil {
				return err
			}
			v64 := uint64(v32)
			v64 |= uint64(0xffffffff) << 32
			cpu.F[instr.Rd] = math.Float64frombits(v64)

		case isa.Fsd:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v := math.Float64bits(cpu.F[instr.Rs2])
			if err := cpu.PutUserUint64(addr, v); err != nil {
				return err
			}

		case isa.Fsw:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v := math.Float32bits(float32(cpu.F[instr.Rs2]))
			if err := cpu.PutUserUint32(addr, uint64(v)); err != nil {
				return err
			}

		case isa.Fence:

		case isa.FeqS:
			b1 := math.Float64bits(cpu.F[instr.Rs1])
			b2 := math.Float64bits(cpu.F[instr.Rs1])

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
			v, err := cpu.UserUint8(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(v)

		case isa.Lb:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v, err := cpu.UserUint8(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int8(v)))

		case isa.Ld:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v, err := cpu.UserUint64(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = v

		case isa.Lhu:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v, err := cpu.UserUint16(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(v)

		case isa.Lui:
			cpu.X[instr.Rd] = uint64(instr.Imm)

		case isa.Lw:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v, err := cpu.UserUint32(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int32(v)))

		case isa.Lwu:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v, err := cpu.UserUint32(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(v)

		case isa.Mul:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] * cpu.X[instr.Rs2]

		case isa.Mulh:
			hi, _ := bits.Mul64(cpu.X[instr.Rs1], cpu.X[instr.Rs2])
			cpu.X[instr.Rd] = hi

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
				cpu.X[instr.Rd] = cpu.X[instr.Rs1]
			} else {
				cpu.X[instr.Rd] = uint64(uint32(cpu.X[instr.Rs1]) %
					uint32(cpu.X[instr.Rs2]))
			}

		case isa.Remw:
			if cpu.X[instr.Rs2] == 0 {
				cpu.X[instr.Rd] = cpu.X[instr.Rs1]
			} else {
				cpu.X[instr.Rd] = uint64(int64(int32(cpu.X[instr.Rs1]) %
					int32(cpu.X[instr.Rs2])))
			}

		case isa.Sb:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			if err := cpu.PutUserUint8(addr, cpu.X[instr.Rs2]); err != nil {
				return err
			}

		case isa.Sd:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			if err := cpu.PutUserUint64(addr, cpu.X[instr.Rs2]); err != nil {
				return err
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
				(cpu.X[instr.Rs2] & 0b111111))

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
			if err := cpu.PutUserUint16(addr, cpu.X[instr.Rs2]); err != nil {
				return err
			}

		case isa.Sw:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			if err := cpu.PutUserUint32(addr, cpu.X[instr.Rs2]); err != nil {
				return err
			}

		case isa.Xor:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] ^ cpu.X[instr.Rs2]

		case isa.Xori:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] ^ uint64(int64(instr.Imm))

			// Control and Status Registers (CSRs).
		case isa.Csrrs:
			csr := instr.Raw >> 20
			switch csr {
			case 0xc01: // time - RDCYCLE instruction
				v := time.Now().Nanosecond()
				cpu.X[instr.Rd] = uint64(v)
				// XX check what to do with R[Rs1]

			default:
				return cpu.Trap(CauseIllegalInstr, cpu.PC, nil)
			}

			// Atomic (A extension).

		case isa.AmoswapD:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.UserUint64(addr)
			if err != nil {
				return err
			}
			err = cpu.PutUserUint64(addr, cpu.X[instr.Rs2])
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = v

		case isa.AmoswapW:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.UserUint32(addr)
			if err != nil {
				return err
			}
			err = cpu.PutUserUint32(addr, cpu.X[instr.Rs2])
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int32(v)))

		case isa.AmoaddD:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.UserUint64(addr)
			if err != nil {
				return err
			}
			t := v + uint64(cpu.X[instr.Rs2])
			err = cpu.PutUserUint64(addr, t)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = v

		case isa.AmoaddW:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.UserUint32(addr)
			if err != nil {
				return err
			}
			t := uint64(int64(int32(v) + int32(cpu.X[instr.Rs2])))
			err = cpu.PutUserUint32(addr, t)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int32(v)))

		case isa.AmoandW:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.UserUint32(addr)
			if err != nil {
				return err
			}
			t := uint64(int64(int32(v) & int32(cpu.X[instr.Rs2])))
			err = cpu.PutUserUint32(addr, t)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int32(v)))

		case isa.AmoorW:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.UserUint32(addr)
			if err != nil {
				return err
			}
			t := uint64(int64(int32(v) | int32(cpu.X[instr.Rs2])))
			err = cpu.PutUserUint32(addr, t)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int32(v)))

		case isa.LrD:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.UserUint64(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = v

		case isa.LrW:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.UserUint32(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int32(v)))

		case isa.ScD:
			addr := cpu.X[instr.Rs1]
			err := cpu.PutUserUint64(addr, cpu.X[instr.Rs2])
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = 0

		case isa.ScW:
			addr := cpu.X[instr.Rs1]
			err := cpu.PutUserUint32(addr, cpu.X[instr.Rs2])
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
			cpu.F[instr.Rd] = float64(cpu.X[instr.Rs1])

		case isa.FcvtWD:
			// XXX If the floating-point value is too large to fit
			// into a 32-bit signed integer the instruction returns
			// the largest possible 32-bit integer and sets an
			// "invalid operation" flag in the fcsr (Floating-point
			// Control and Status Register).
			cpu.X[instr.Rd] = uint64(int64(int32(cpu.F[instr.Rs1])))

		case isa.FcvtLD:
			// XXX If the value is out of range, fcsr.fflags.NV is set
			// to 1
			cpu.X[instr.Rd] = uint64(int64(cpu.F[instr.Rs1]))

		default:
			return fmt.Errorf("instruction %v[0x%x] not implemented yet",
				instr, instr.Raw)
		}
		cpu.PC += uint64(size)
	}
}
