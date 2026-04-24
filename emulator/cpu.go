//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package emulator

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/bits"

	"github.com/markkurossi/riscv/isa"
)

var (
	bo = binary.LittleEndian
)

type Syscall func(cpu *CPU, id, a0, a1, a2, a3, a4, a5 uint64) (
	uint64, error)

type CPU struct {
	// Registers x0-x31.
	X  [32]uint64
	F  [32]float64
	PC uint64

	// Instruction count
	IC uint64

	Mem     *Memory
	Syscall Syscall
}

func (cpu *CPU) Errorf(msg string, args ...interface{}) error {
	err := fmt.Errorf(msg, args...)
	fmt.Println(err.Error())

	fmt.Printf("epc : %016x ra : %016x sp : %016x\n",
		cpu.PC, cpu.X[isa.Ra], cpu.X[isa.Sp])
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
	fmt.Printf(" s11: %016x t3 : %016x t4: %016x\n",
		cpu.X[isa.S11], cpu.X[isa.T3], cpu.X[isa.T4])
	fmt.Printf(" t5 : %016x t6 : %016x\n",
		cpu.X[isa.T5], cpu.X[isa.T6])

	return err
}

func (cpu *CPU) Run() error {
	for {
		cpu.X[isa.Zero] = 0

		seg, ofs, err := cpu.Mem.Map(cpu.PC, 4)
		if err != nil {
			return cpu.Errorf("Unable to handle page fault for address: 0x%08x",
				cpu.PC)
		}
		instr, size, err := isa.Decode(seg.Data[ofs:])
		if err != nil {
			return err
		}
		cpu.IC++

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

		case isa.Divu:
			if cpu.X[instr.Rs2] == 0 {
				// Division by zero → result = -1
				cpu.X[instr.Rd] = ^uint64(0)
			} else {
				cpu.X[instr.Rd] = cpu.X[instr.Rs1] / cpu.X[instr.Rs2]
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
			v, err := cpu.Mem.Load64(addr)
			if err != nil {
				return err
			}
			cpu.F[instr.Rd] = math.Float64frombits(v)

		case isa.Fsd:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v := math.Float64bits(cpu.F[instr.Rs2])
			if err := cpu.Mem.Store64(addr, v); err != nil {
				return err
			}

		case isa.Fence:

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
			v, err := cpu.Mem.Load8(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(v)

		case isa.Lb:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v, err := cpu.Mem.Load8(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int8(v)))

		case isa.Ld:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v, err := cpu.Mem.Load64(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = v

		case isa.Lhu:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v, err := cpu.Mem.Load16(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(v)

		case isa.Lui:
			cpu.X[instr.Rd] = uint64(instr.Imm)

		case isa.Lw:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v, err := cpu.Mem.Load32(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int32(v)))

		case isa.Lwu:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v, err := cpu.Mem.Load32(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(v)

		case isa.Mul:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] * cpu.X[instr.Rs2]

		case isa.Mulhu:
			hi, _ := bits.Mul64(cpu.X[instr.Rs1], cpu.X[instr.Rs2])
			cpu.X[instr.Rd] = hi

		case isa.Or:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] | cpu.X[instr.Rs2]

		case isa.Ori:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] | uint64(int64(instr.Imm))

		case isa.Remw:
			if cpu.X[instr.Rs2] == 0 {
				cpu.X[instr.Rd] = cpu.X[instr.Rs1]
			} else {
				cpu.X[instr.Rd] = uint64(int64(int32(cpu.X[instr.Rs1]) %
					int32(cpu.X[instr.Rs2])))
			}

		case isa.Remu:
			if cpu.X[instr.Rs2] == 0 {
				cpu.X[instr.Rd] = cpu.X[instr.Rs1]
			} else {
				cpu.X[instr.Rd] = cpu.X[instr.Rs1] % cpu.X[instr.Rs2]
			}

		case isa.Sb:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			if err := cpu.Mem.Store8(addr, cpu.X[instr.Rs2]); err != nil {
				return err
			}

		case isa.Sd:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			if err := cpu.Mem.Store64(addr, cpu.X[instr.Rs2]); err != nil {
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

		case isa.Srl:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] >> (cpu.X[instr.Rs2] & 0b111111)

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
			if err := cpu.Mem.Store16(addr, cpu.X[instr.Rs2]); err != nil {
				return err
			}

		case isa.Sw:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			if err := cpu.Mem.Store32(addr, cpu.X[instr.Rs2]); err != nil {
				return err
			}

		case isa.Xor:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] ^ cpu.X[instr.Rs2]

		case isa.Xori:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] ^ uint64(instr.Imm)

			// Atomic (A extension).

		case isa.AmoswapD:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.Mem.Load64(addr)
			if err != nil {
				return err
			}
			err = cpu.Mem.Store64(addr, cpu.X[instr.Rs2])
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = v

		case isa.AmoswapW:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.Mem.Load32(addr)
			if err != nil {
				return err
			}
			err = cpu.Mem.Store32(addr, cpu.X[instr.Rs2])
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int32(v)))

		case isa.AmoaddD:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.Mem.Load64(addr)
			if err != nil {
				return err
			}
			t := v + uint64(cpu.X[instr.Rs2])
			err = cpu.Mem.Store64(addr, t)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = v

		case isa.AmoaddW:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.Mem.Load32(addr)
			if err != nil {
				return err
			}
			t := uint64(int64(int32(v) + int32(cpu.X[instr.Rs2])))
			err = cpu.Mem.Store32(addr, t)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int32(v)))

		case isa.LrD:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.Mem.Load64(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = v

		case isa.LrW:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.Mem.Load32(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int32(v)))

		case isa.ScD:
			addr := cpu.X[instr.Rs1]
			err := cpu.Mem.Store64(addr, cpu.X[instr.Rs2])
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = 0

		case isa.ScW:
			addr := cpu.X[instr.Rs1]
			err := cpu.Mem.Store32(addr, cpu.X[instr.Rs2])
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = 0

		default:
			return cpu.Errorf("Instruction %v[0x%x] not implemented yet",
				instr, instr.Raw)
		}
		cpu.PC += uint64(size)
	}
}

func Error(errno Errno) uint64 {
	return uint64(int64(-errno))
}
