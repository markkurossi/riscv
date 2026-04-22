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
	for {
		cpu.X[isa.Zero] = 0

		seg, ofs, err := cpu.Mem.Map(cpu.PC, 4)
		if err != nil {
			return fmt.Errorf("invalid PC %x: %v", cpu.PC, err)
		}
		instr, size, err := isa.Decode(seg.Data[ofs:])
		if err != nil {
			return err
		}

		fmt.Printf("%8x:\t%08x\t%v\n", cpu.PC, instr.Raw, instr)

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
			// But RISC-V requires:
			// - div by zero → result = -1
			// - rem by zero → result = dividend
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] / cpu.X[instr.Rs2]

		case isa.Divw:
			// But RISC-V requires:
			// - div by zero → result = -1
			// - rem by zero → result = dividend
			cpu.X[instr.Rd] = uint64(int64(int32(cpu.X[instr.Rs1]) /
				int32(cpu.X[instr.Rs2])))

		case isa.Ecall:
			if err = cpu.ecall(); err != nil {
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

		case isa.Mul:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] * cpu.X[instr.Rs2]

		case isa.Remw:
			cpu.X[instr.Rd] = uint64(int64(int32(cpu.X[instr.Rs1]) %
				int32(cpu.X[instr.Rs2])))

		case isa.Remu:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] % cpu.X[instr.Rs2]

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

		case isa.Slli:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] << instr.Imm

		case isa.Slliw:
			cpu.X[instr.Rd] = uint64(int64(int32(cpu.X[instr.Rs1]) <<
				instr.Imm))

		case isa.Srli:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] >> instr.Imm

		case isa.Sub:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] - cpu.X[instr.Rs2]

		case isa.Sw:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			if err := cpu.Mem.Store32(addr, cpu.X[instr.Rs2]); err != nil {
				return err
			}

		case isa.Xori:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] ^ uint64(instr.Imm)

		default:
			return fmt.Errorf("cpu: instrs %v not implemented yet", instr)
		}
		cpu.PC += uint64(size)
	}
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

	case 96: // set_tid_address
		cpu.X[isa.A0] = 0 // caller's tread ID

	case 99: // set_robust_list
		cpu.X[isa.A0] = 0

	case 214: // brk
		if cpu.X[isa.A0] == 0 {
			cpu.X[isa.A0] = cpu.Mem.HeapEnd
			fmt.Printf("       brk(0) => %x\n", cpu.Mem.HeapEnd)
		} else if cpu.X[isa.A0] > cpu.Mem.HeapEnd {
			// Compute brk.
			brk := (cpu.X[isa.A0] + 4095) & ^uint64(0xfff)

			// Get current segment.
			seg, _, err := cpu.Mem.Map(cpu.Mem.HeapStart, 8)
			if err != nil {
				// Create memory.
				seg = &Segment{
					Start: cpu.Mem.HeapStart,
					End:   brk,
					Data:  make([]byte, brk-cpu.Mem.HeapStart),
					Read:  true,
					Write: true,
				}
				cpu.Mem.Add(seg)
			} else {
				// Extend current segment.
				n := make([]byte, brk-cpu.Mem.HeapStart)
				copy(n, seg.Data)
				seg.Data = n
				seg.End = brk
			}
			fmt.Printf("       brk(%x) => %x\n", cpu.X[isa.A0], brk)

			cpu.Mem.HeapEnd = brk
			cpu.X[isa.A0] = brk
		}

	default:
		return fmt.Errorf("unsupported syscall %v", cpu.X[isa.A7])
	}

	return nil
}
