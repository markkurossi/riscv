//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package emulator

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/markkurossi/riscv/isa"
)

var (
	bo = binary.LittleEndian
)

type CPU struct {
	// Registers x0-x31.
	X  [32]uint64
	PC uint64

	Mem *Memory
}

func (cpu *CPU) Run() error {
	for count := 0; count < 30; count++ {
		cpu.X[isa.Zero] = 0

		seg, ofs, err := cpu.Mem.Map(cpu.PC, 4)
		if err != nil {
			return fmt.Errorf("invalid PC %x: %v", cpu.PC, err)
		}
		instr, size, err := isa.Decode(seg.Data[ofs:], cpu.PC)
		if err != nil {
			return err
		}

		fmt.Printf("%8x:\t%08x\t%v\n", cpu.PC, instr.Raw, instr)

		switch instr.Op {
		case isa.Addi:
			cpu.X[instr.Rd] = uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))

		case isa.Bge:
			if int64(cpu.X[instr.Rs1]) >= int64(cpu.X[instr.Rs2]) {
				cpu.PC = uint64(int64(cpu.PC) + int64(instr.Imm))
				size = 0
			}

		case isa.Bne:
			if cpu.X[instr.Rs1] != cpu.X[instr.Rs2] {
				cpu.PC = uint64(int64(cpu.PC) + int64(instr.Imm))
				size = 0
			}

		case isa.Ecall:
			if err = cpu.ecall(); err != nil {
				return err
			}

		case isa.Jal:
			cpu.X[instr.Rd] = cpu.PC + 4
			cpu.PC = uint64(int64(cpu.PC) + int64(instr.Imm))
			size = 0

		case isa.Jalr:
			t := cpu.PC + 4
			cpu.PC = uint64(int64(cpu.X[instr.Rs1])+int64(instr.Imm)) &^ 1
			cpu.X[instr.Rd] = t
			size = 0

		case isa.Ld:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v, err := cpu.Mem.Load64(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = v

		case isa.Lui:
			cpu.X[instr.Rd] = uint64(instr.Imm << 12)

		case isa.Sd:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			if err := cpu.Mem.Store64(addr, cpu.X[instr.Rs2]); err != nil {
				return err
			}

		case isa.Sw:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			if err := cpu.Mem.Store32(addr, cpu.X[instr.Rs2]); err != nil {
				return err
			}

		default:
			fmt.Printf("cpu: instrs %v not implemented yet\n", instr)
		}
		cpu.PC += uint64(size)
	}

	return nil
}

func (cpu *CPU) ecall() error {
	syscall := cpu.X[isa.A7]
	info, ok := SyscallInfo[syscall]
	if !ok || info.Argc == 0 {
		fmt.Printf("ecall: %v(%v,%v,%v)\n",
			syscall, cpu.X[isa.A0], cpu.X[isa.A1], cpu.X[isa.A2])
	} else if len(info.Format) > 0 {
		fmt.Printf("ecall: %s(", info.Name)
		for idx, ch := range info.Format {
			if idx > 0 {
				fmt.Print(",")
			}
			arg := cpu.X[int(isa.A0)+idx]

			switch ch {
			case 'p':
				fmt.Printf("%x", arg)
			default:
				fmt.Printf("%v", arg)
			}
		}
		fmt.Println(")")
	} else {
		fmt.Printf("ecall: %s(", info.Name)
		for i := 0; i < info.Argc; i++ {
			if i > 0 {
				fmt.Print(",")
			}
			fmt.Printf("%v", cpu.X[int(isa.A0)+i])
		}
		fmt.Println(")")
	}

	switch cpu.X[isa.A7] {
	case 64: // write
		fd := cpu.X[isa.A0]
		addr := cpu.X[isa.A1]
		len := cpu.X[isa.A2]

		_ = fd

		var i uint64

		for i = 0; i < len; i++ {
			b, err := cpu.Mem.Load8(addr + i)
			if err != nil {
				return err
			}
			os.Stdout.Write([]byte{b})
			if err != nil {
				break
			}
		}
		if i < len {
			cpu.X[isa.A0] = ^uint64(0)
		} else {
			cpu.X[isa.A0] = len
		}

	case 93: // exit
		os.Exit(int(cpu.X[isa.A0]))

	default:
		return fmt.Errorf("unsupported syscall %v", cpu.X[isa.A7])
	}

	return nil
}
