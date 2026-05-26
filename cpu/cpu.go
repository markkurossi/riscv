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
	"sync"
	"time"

	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/memory"
	"github.com/markkurossi/riscv/mmu"
)

var (
	bo          = binary.LittleEndian
	_  isa.Hart = &CPU{}
)

const (
	cpuDebug = false
	cpuColor = false
)

type TrapHandler func(cpu *CPU, trap *isa.Trap) (bool, error)

type CPU struct {
	Trace bool

	X [32]uint64
	F [32]float64

	mode     isa.PrivilegeMode
	shutdown bool

	CSR     [4096]uint64
	mstatus isa.Mstatus

	PC uint64

	Reservation    uint64 // The address currently reserved
	HasReservation bool   // Whether the reservation is active

	decodeCache [4096]struct {
		Raw   uint32
		Instr isa.Instr
	}

	Time    uint64
	Instret uint64

	m          sync.Mutex
	c          *sync.Cond
	wfiTimeout bool

	StartTime time.Time
	Runtime   time.Duration

	MMU *mmu.MMU

	TrapHandler TrapHandler
	Symtab      Symtab

	lastDescOp isa.Op
	DebugTrace bool
	LastSymbol *SymEntry
}

func New(mem *memory.Memory) *CPU {
	cpu := &CPU{
		MMU: &mmu.MMU{
			Mem: mem,
		},
	}
	cpu.c = sync.NewCond(&cpu.m)
	cpu.MMU.Hart = cpu

	return cpu
}

func (cpu *CPU) Now() uint64 {
	return cpu.Time
}

func (cpu *CPU) syncTime() uint64 {
	cpu.Time = uint64(time.Since(cpu.StartTime).Nanoseconds()) / 10
	return cpu.Time
}

func (cpu *CPU) Mode() isa.PrivilegeMode {
	return cpu.mode
}

func (cpu *CPU) SetMode(mode isa.PrivilegeMode) {
	cpu.mode = mode
}

func (cpu *CPU) Run() error {
	cpu.StartTime = time.Now()

	for !cpu.shutdown {
		err := cpu.loop()
		if err != nil {
			if trap, ok := errors.AsType[*isa.Trap](err); ok {
				// The trap handler saved relevant CPU state and moved
				// PC to trap handler. All done, let's continue
				if false {
					switch trap.Cause {
					case isa.CauseEcallU, isa.CauseEcallS,
						isa.CauseEcallVS, isa.CauseEcallM:

					default:
						fmt.Printf("CPU: trap %v\n", trap)
						if trap.Err != nil {
							fmt.Printf("  caused by %v\n", trap.Err)
						}
						cpu.Dump(trap.PC)
						cpu.disassembleKernel(trap.PC)
					}
				}
			} else {
				return fmt.Errorf("CPU: panic: %w", err)
			}
		}
	}
	cpu.Runtime = time.Since(cpu.StartTime)

	// Halt all memory-mapped devices.
	return cpu.MMU.ROM.Halt()
}

func (cpu *CPU) Shutdown() {
	cpu.shutdown = true
}

func (cpu *CPU) loop() error {
	if cpu.PC%2 == 1 {
		return cpu.Trap(isa.CauseInstAddrMisaligned, cpu.PC, nil)
	}

	var codePagenum uint64
	var codePage []byte

dispatch:
	for {
		var instr isa.Instr
		var err error
		var size int

		// Ensure monotonic clock, this is slower than real clock so
		// we won't jump forward.
		cpu.Time++

		// Ensure zero=zero.
		cpu.X[isa.Zero] = 0

		// Check interrupts every 64 instructions or if any interrupts
		// are pending. The loop below will not trigger interrupts if
		// they are pending but not enabled.
		// XXX consider moving CsrMip and CsrMie to local variables.
		if cpu.Instret&0x3f == 0 || cpu.CSR[CsrMip] != 0 {
			// Sync time to wall clock.
			now := cpu.syncTime()

			mip := cpu.CSR[CsrMip]
			stimecmp := cpu.CSR[CsrStimecmp]

			// Check timer interrupts.
			if now >= stimecmp {
				mip |= isa.IntSTIP
			}
			cpu.CSR[CsrMip] = mip

			mie := cpu.CSR[CsrMie]
			pending := mip & mie

			if pending != 0 {
				mideleg := cpu.CSR[CsrMideleg]
				currentMode := cpu.Mode()

				// Check each pending interrupt, highest priority first
				for _, bit := range []uint64{11, 9, 7, 5, 3, 1} {
					if pending&(1<<bit) == 0 {
						continue
					}

					if mideleg&(1<<bit) == 0 {
						// Handle in M-mode Enabled if explicitly in a
						// lower mode, OR if in M-mode with MIE active.
						if currentMode < isa.ModeM ||
							(currentMode == isa.ModeM && cpu.mstatus.MIE()) {
							cpu.Interrupt(isa.ModeM, bit)
							continue dispatch
						}
					} else {
						// Delegated to S-mode Enabled if explicitly
						// in User mode, OR if in S-mode with SIE
						// active.
						if currentMode < isa.ModeS ||
							(currentMode == isa.ModeS && cpu.mstatus.SIE()) {
							cpu.Interrupt(isa.ModeS, bit)
							continue dispatch
						}
					}
				}
			}
			if cpu.shutdown {
				// Jump to trampoline to terminate the CPU loop.
				return nil
			}
		}

		if memory.Page(cpu.PC) != codePagenum {
			paddr, err := cpu.MMU.Map(cpu.PC, mmu.AccessExec)
			if err != nil {
				return err
			}
			codePage, err = cpu.MMU.Mem.Page(memory.Page(paddr))
			if err != nil {
				return cpu.Trap(isa.CauseInstAccessFault, paddr, err)
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
					return cpu.Trap(isa.CauseInstAccessFault, paddr, err)
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

		cpu.Instret++

		if cpuDebug || cpu.DebugTrace {
			if cpu.Symtab != nil {
				mapped, entry := cpu.kernelMap(cpu.PC)
				if entry != nil && entry != cpu.LastSymbol {
					fmt.Printf("%v  <%s+0x%x>:\r\n",
						fmtAddr(cpu.PC), entry.Name, mapped-entry.Start)
					cpu.LastSymbol = entry
				}
				if entry != nil {
					switch entry.Name {
					case "__delay":
						if true {
							// cpu.MMU.Mem.Strings()
							os.Exit(1)
						}
					}
				}
			}
			cpu.trace(raw, instr, "")
		}

		switch instr.Op {
		case isa.Invalid:
			return fmt.Errorf("invalid instruction: %v", instr.Op)

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
				cpu.traceFunc(cpu.PC)
				cpu.tracef(raw, instr, "")
			}
			return cpu.Trap(isa.CauseBreakpoint, 0, nil)

		case isa.Sret:
			cpu.mstatus.SetSIE(cpu.mstatus.SPIE())
			cpu.mstatus.SetSPIE(true)
			cpu.SetMode(cpu.mstatus.SPP())
			cpu.mstatus.SetSPP(isa.ModeU)
			cpu.PC = cpu.CSR[CsrSepc]

			if cpu.Trace {
				cpu.traceFunc(cpu.PC)
				cpu.tracef(raw, instr, "sepc=%x, mode=%v", cpu.PC, cpu.Mode())
			}

			continue

		case isa.Mret:
			if cpu.Trace {
				cpu.traceFunc(cpu.PC)
				cpu.tracef(raw, instr, "mode=%v => %v",
					cpu.Mode(), cpu.mstatus.MPP())
			}

			cpu.mstatus.SetMIE(cpu.mstatus.MPIE())
			cpu.mstatus.SetMPIE(true)
			cpu.SetMode(cpu.mstatus.MPP())
			cpu.mstatus.SetMPP(isa.ModeU)
			cpu.PC = cpu.CSR[CsrMepc]

			if cpu.Trace {
				cpu.tracef(raw, instr, "mepc=%x, mode=%v", cpu.PC, cpu.Mode())
			}

			continue

		case isa.Ecall:
			var cause uint64
			switch cpu.Mode() {
			case isa.ModeU:
				cause = isa.CauseEcallU
			case isa.ModeS:
				cause = isa.CauseEcallS
			case isa.ModeM:
				cause = isa.CauseEcallM
			default:
				return cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
					fmt.Errorf("ecall in %v-mode", cpu.Mode()))
			}
			if cpu.Trace {
				cpu.traceFunc(cpu.PC)
				cpu.tracef(raw, instr,
					"a7=%x, a6=%x, a0=%x, a1=%x",
					cpu.X[isa.A7], cpu.X[isa.A6],
					cpu.X[isa.A0], cpu.X[isa.A1])
			}

			err = cpu.trap(cpu.PC, cause, 0, nil)
			if err != nil {
				return err
			}
			continue

		case isa.Wfi:
			// Calculate delay to the next stimecmp interrupt.

			stimecmp := cpu.CSR[CsrStimecmp]
			now := cpu.Now()

			if stimecmp == 0xffffffffffffffff || now >= stimecmp {
				break
			}
			cpu.m.Lock()
			cpu.wfiTimeout = false
			cpu.m.Unlock()

			delay := time.Duration(stimecmp-now) * 10
			go func() {
				time.Sleep(delay)
				cpu.m.Lock()
				cpu.wfiTimeout = true
				cpu.c.Broadcast()
				cpu.m.Unlock()
			}()

			// Wait for interrupt.
			cpu.m.Lock()
			for cpu.CSR[CsrMip]&cpu.CSR[CsrMie] == 0 && !cpu.wfiTimeout {
				cpu.c.Wait()
			}
			cpu.m.Unlock()

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
			if cpu.DebugTrace {
				cpu.traceFunc(cpu.PC)
			}

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

			if cpu.mode <= isa.ModeS && tlb.VPN == vpn &&
				tlb.Flags.Readable() && memory.Avail(addr, 8) {
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

			if cpu.mode <= isa.ModeS && tlb.VPN == vpn &&
				tlb.Flags.Writable() && memory.Avail(addr, 8) {
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

		case isa.Sra:
			cpu.X[instr.Rd] = uint64(int64(cpu.X[instr.Rs1]) >>
				cpu.X[instr.Rs2] & 0x3f)

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
			cpu.X[instr.Rd] = uint64(int64(int32(uint32(cpu.X[instr.Rs1] -
				cpu.X[instr.Rs2]))))

		case isa.Xor:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] ^ cpu.X[instr.Rs2]

		case isa.Xori:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] ^ uint64(int64(instr.Imm))

			// Control and Status Registers (CSRs).
		case isa.Csrrs:
			csr := CSR(instr.Imm)
			t, err := cpu.GetCSR(csr)
			if err != nil {
				return err
			}
			if instr.Rs1 != isa.Zero {
				err = cpu.SetCSRX(csr, t|cpu.X[instr.Rs1], raw, instr)
				if err != nil {
					return err
				}
			}
			cpu.X[instr.Rd] = t

		case isa.Csrrc:
			csr := CSR(instr.Imm)
			t, err := cpu.GetCSR(csr)
			if err != nil {
				return err
			}
			if instr.Rs1 != isa.Zero {
				err = cpu.SetCSRX(csr, t & ^cpu.X[instr.Rs1], raw, instr)
				if err != nil {
					return err
				}
			}
			cpu.X[instr.Rd] = t

		case isa.Csrrci:
			csr := CSR(instr.Imm)
			t, err := cpu.GetCSR(csr)
			if err != nil {
				return err
			}
			if instr.Rs1 != isa.Zero {
				err = cpu.SetCSRX(csr, t & ^uint64(instr.Rs1), raw, instr)
				if err != nil {
					return err
				}
			}
			cpu.X[instr.Rd] = t

		case isa.Csrrsi:
			csr := CSR(instr.Imm)
			t, err := cpu.GetCSR(csr)
			if err != nil {
				return err
			}
			err = cpu.SetCSRX(csr, t|uint64(instr.Rs1), raw, instr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = t

		case isa.Csrrw:
			csr := CSR(instr.Imm)
			oldCSR, err := cpu.GetCSR(csr) // 1. Capture old CSR value
			if err != nil {
				return err
			}
			valToSet := cpu.X[instr.Rs1] // 2. Capture value from GPR

			err = cpu.SetCSRX(csr, valToSet, raw, instr) // 3. Update CSR
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = oldCSR // 4. Update GPR with old CSR

		case isa.Csrrwi:
			csr := CSR(instr.Imm)
			err = cpu.SetCSRX(csr, uint64(instr.Rs1), raw, instr)
			if err != nil {
				return err
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
				return cpu.Trap(isa.CauseStoreAddrMisaligned, addr, nil)
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

		case isa.AmoxorD:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.MMU.Load64(addr)
			if err != nil {
				return err
			}
			t := v ^ cpu.X[instr.Rs2]
			err = cpu.MMU.Store64(addr, t)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = v

		case isa.AmoxorW:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.MMU.Load32(addr)
			if err != nil {
				return err
			}
			t := uint64(int64(int32(v) ^ int32(cpu.X[instr.Rs2])))
			err = cpu.MMU.Store32(addr, t)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int32(v)))

		case isa.AmomaxuD:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.MMU.Load64(addr)
			if err != nil {
				return err
			}
			t := cpu.X[instr.Rs2]
			if v > t {
				t = v
			}
			err = cpu.MMU.Store64(addr, t)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = v

		case isa.AmomaxuW:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.MMU.Load32(addr)
			if err != nil {
				return err
			}
			t := uint32(cpu.X[instr.Rs2])
			if v > t {
				t = v
			}
			err = cpu.MMU.Store32(addr, uint64(t))
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(v)

		case isa.LrW:
			addr := cpu.X[instr.Rs1]
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

		case isa.FsgnjD:
			v := math.Float64bits(cpu.F[instr.Rs1])
			b := math.Float64bits(cpu.F[instr.Rs2])

			v &^= 1 << 63
			v |= b & (1 << 63)

			cpu.F[instr.Rd] = math.Float64frombits(v)

		default:
			cpu.tracef(raw, instr, "not implemented")
			cpu.Dump(cpu.PC)
			return fmt.Errorf("instruction %v[0x%x] not implemented yet",
				instr, raw)
		}
		cpu.PC += uint64(size)
	}
}

func (cpu *CPU) ClearInterrupt(mask uint64) {
	cpu.m.Lock()
	cpu.CSR[CsrMip] &^= mask
	cpu.m.Unlock()
}

func (cpu *CPU) SetInterrupt(mask uint64) {
	cpu.m.Lock()
	cpu.CSR[CsrMip] |= mask
	cpu.m.Unlock()
	cpu.c.Broadcast()
}

func (cpu *CPU) FuncName(pc uint64) (*SymEntry, uint64) {
	if cpu.Symtab == nil {
		return nil, 0
	}
	// OpenSBI range: no kernel symbols here
	if pc >= 0x80000000 && pc < 0x80200000 {
		return nil, 0
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
		return nil, 0
	}
	return entry, mapped
}

func (cpu *CPU) traceFunc(pc uint64) *SymEntry {
	entry, mapped := cpu.FuncName(pc)
	if entry == nil {
		return nil
	}

	cpu.ColorOn()
	fmt.Printf("%8x  <%s+0x%x>:", pc, entry.Name, mapped-entry.Start)
	cpu.ColorOff()
	fmt.Print("\r\n")

	return entry
}

func (cpu *CPU) tracef(raw uint32, instr isa.Instr,
	format string, args ...interface{}) {

	cpu.trace(raw, instr, fmt.Sprintf(format, args...))
}

func (cpu *CPU) ColorOn() {
	if !cpuColor {
		return
	}
	var color string
	if true {
		switch cpu.Mode() {
		case isa.ModeS:
			color = "30;106" // black/bright cyan
		case isa.ModeM:
			color = "30;103" // black/bright yellow
		default:
			color = "30" // black/white
		}
	} else {
		switch cpu.Mode() {
		case isa.ModeS:
			color = "1;32" // bright green
		case isa.ModeM:
			color = "1;35" // bright magenta
		default:
			color = "30" // black/white
		}
	}
	fmt.Printf("\x1b[%sm", color)
}

func (cpu *CPU) ColorOff() {
	if !cpuColor {
		return
	}
	fmt.Printf("\x1b[0m")
}

func fmtAddr(addr uint64) string {
	if addr < 0x100000000 {
		return fmt.Sprintf("  %08x", addr)
	} else if addr >= 0xffffffff00000000 {
		if false {
			return fmt.Sprintf("+f%08x", uint32(addr))
		} else {
			return fmt.Sprintf("%16x", addr)
		}
	}
	return fmt.Sprintf("%10x", addr)
}

func (cpu *CPU) trace(raw uint32, instr isa.Instr, msg string) {
	cpu.ColorOn()

	var line string

	addr := fmtAddr(cpu.PC)
	if raw&0b11 == 0b11 {
		line = fmt.Sprintf("%s:  %08x   %v", addr, raw, instr)
	} else {
		line = fmt.Sprintf("%s:  %04x       %v", addr, raw, instr)
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
	fmt.Print(line)
	cpu.ColorOff()
	fmt.Print("\r\n")
}
