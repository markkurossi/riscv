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
			if err = cpu.ecall(); err != nil {
				return err
			}

		case isa.Fsd:
			// XXX floating point

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

		case isa.Lwu:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v, err := cpu.Mem.Load32(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(v)

		case isa.Mul:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] * cpu.X[instr.Rs2]

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
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] << cpu.X[instr.Rs2]

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
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] >> cpu.X[instr.Rs2]

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
			err = cpu.Mem.Store32(addr, t)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = t

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
			cpu.X[instr.Rd] = t

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
			return fmt.Errorf("cpu: instrs %v not implemented yet", instr)
		}
		cpu.PC += uint64(size)
	}
}

func (cpu *CPU) EcallError(errno Errno) error {
	cpu.X[isa.A0] = uint64(int64(-errno))
	return nil
}

func (cpu *CPU) ecall() error {
	syscall := cpu.X[isa.A7]
	info, ok := SyscallInfo[syscall]
	if !ok {
		fmt.Printf("ecall: %v(%v,%v,%v,%v,%v,%v)\n",
			syscall,
			cpu.X[isa.A0], cpu.X[isa.A1], cpu.X[isa.A2],
			cpu.X[isa.A3], cpu.X[isa.A4], cpu.X[isa.A5])
	} else if info.Argc == 0 {
		fmt.Printf("ecall: %v(%v,%v,%v,%v,%v,%v)\n",
			info.Name,
			cpu.X[isa.A0], cpu.X[isa.A1], cpu.X[isa.A2],
			cpu.X[isa.A3], cpu.X[isa.A4], cpu.X[isa.A5])
	} else if len(info.Format) > 0 {
		fmt.Printf("ecall: %s(", info.Name)
		for idx, ch := range info.Format {
			if idx > 0 {
				fmt.Print(",")
			}
			arg := cpu.X[int(isa.A0)+idx]

			switch ch {
			case 'i':
				fmt.Printf("%v", int64(arg))
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

	case 66: // writev
		fd := int(cpu.X[isa.A0])
		iov := cpu.X[isa.A1]
		iovcnt := int(cpu.X[isa.A2])

		var f *os.File
		switch fd {
		case 0:
			f = os.Stdin
		case 1:
			f = os.Stdout
		case 2:
			f = os.Stderr
		default:
			return cpu.EcallError(ErrnoEBADF)
		}

		var wrote uint64

		for i := 0; i < iovcnt; i++ {
			base, err := cpu.Mem.Load64(iov)
			if err != nil {
				return err
			}
			l, err := cpu.Mem.Load64(iov + 8)
			if err != nil {
				return err
			}
			iov += 16

			seg, ofs, err := cpu.Mem.Map(base, int(l))
			if err != nil {
				return err
			}

			n, err := f.Write(seg.Data[ofs : ofs+l])
			if err != nil {
				return err
			}
			wrote += uint64(n)
		}
		cpu.X[isa.A0] = wrote

	case 78: // readlinkat
		const AtFdcwd int64 = -100
		arg0 := int64(cpu.X[isa.A0])
		if arg0 == AtFdcwd {
			fmt.Printf("     - AT_FDCWD\n")
		}
		cpu.X[isa.A0] = ^uint64(0)

	case 80: // fstat
		cpu.X[isa.A0] = ^uint64(0)

	case 93: // exit
		os.Exit(int(cpu.X[isa.A0]))

	case 94: // exit_group
		os.Exit(int(cpu.X[isa.A0]))

	case 96: // set_tid_address
		cpu.X[isa.A0] = 1000 // caller's tread ID

	case 98: // futex
		addr := cpu.X[isa.A0]
		op := cpu.X[isa.A1]
		val := cpu.X[isa.A2]

		var opName string

		switch op & 127 {
		case 0:
			opName = "FUTEX_WAIT"
		case 1:
			opName = "FUTEX_WAKE"
		case 2:
			opName = "FUTEX_FD"
		case 3:
			opName = "FUTEX_REQUEUE"
		case 4:
			opName = "FUTEX_CMP_REQUEUE"
		case 5:
			opName = "FUTEX_WAKE_OP"
		case 6:
			opName = "FUTEX_LOCK_PI"
		case 7:
			opName = "FUTEX_UNLOCK_PI"
		case 8:
			opName = "FUTEX_TRYLOCK_PI"
		case 9:
			opName = "FUTEX_WAIT_BITSET"
		case 10:
			opName = "FUTEX_WAKE_BITSET"
		case 11:
			opName = "FUTEX_WAIT_REQUEUE_PI"
		case 12:
			opName = "FUTEX_CMP_REQUEUE_PI"
		case 13:
			opName = "FUTEX_LOCK_PI2"
		}

		fmt.Printf("    => futex(%x,%v[%v],%v)\n", addr, op, opName, val)
		if op&127 == 0 {
			v, err := cpu.Mem.Load32(addr)
			if err != nil {
				return err
			}
			fmt.Printf("    => val=%v, wait=%v\n", v, val)

			if true {
				return fmt.Errorf("futex debug")
			}

			if uint64(v) >= val {
				cpu.X[isa.A0] = 0
			} else {
				cpu.EcallError(ErrnoEAGAIN)
			}
		} else {
			cpu.X[isa.A0] = ^uint64(0)
		}

	case 99: // set_robust_list
		cpu.X[isa.A0] = 0

	case 214: // brk
		if cpu.X[isa.A0] == 0 {
			cpu.X[isa.A0] = cpu.Mem.HeapEnd
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

			cpu.Mem.HeapEnd = brk
			cpu.X[isa.A0] = brk
		}

	case 215: // munmap
		// XXX check if the region was mmap'ed
		cpu.X[isa.A0] = 0

	case 222: // mmap
		length := cpu.X[isa.A1]
		prot := cpu.X[isa.A2]
		flags := cpu.X[isa.A3]

		_ = flags

		if cpu.X[isa.A0] == 0 {
			// Choose address from the mmap region
			addr := cpu.Mem.MmapEnd

			// Align size to page size.
			length = (length + 4095) &^ 4095

			// Create the segment
			seg := &Segment{
				Start: addr,
				End:   addr + length,
				Data:  make([]byte, length),
				Read:  (prot & 1) != 0, // PROT_READ
				Write: (prot & 2) != 0, // PROT_WRITE
			}
			cpu.Mem.Add(seg)

			// Update pointer for next call.
			cpu.Mem.MmapEnd += length

			fmt.Printf("    => %x:%x\n", addr, addr+length)

			// Return the allocated address in A0
			cpu.X[isa.A0] = addr
		} else {
			if true {
				return fmt.Errorf("mmap: unsupported flow")
			}
			return cpu.EcallError(ErrnoEINVAL)
		}

	case 226: // mprotec
		cpu.X[isa.A0] = 0

	case 261: // prlimit64
		cpu.X[isa.A0] = 0

	case 278: // getrandom
		cpu.X[isa.A0] = cpu.X[isa.A1]

	default:
		if false {
			return fmt.Errorf("unsupported syscall %v", cpu.X[isa.A7])
		} else {
			fmt.Printf("    => skipping syscall %v\n", cpu.X[isa.A7])
		}
		cpu.X[isa.A0] = 0
	}

	return nil
}
