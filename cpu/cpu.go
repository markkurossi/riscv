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
	"log"
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

	vpu *VPU

	// A (Atomic) Extension - reserved memory access.
	Reservation      uint64
	ReservationValid bool

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

	MMU         *mmu.MMU
	codePagenum uint64
	codePage    []byte

	TrapHandler TrapHandler
	Symtab      Symtab

	CSR7c2Filename string
	csr7c2File     *os.File
	csr7c2Refcount int

	lastDescOp  isa.Op
	DebugTrace  bool
	DebugTrace2 bool
	LastSymbol  *SymEntry
}

func New(mem *memory.Memory) *CPU {
	cpu := &CPU{
		vpu: NewVPU(),
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
	// The CPU.Time is a monotonically increasing counter that is
	// incremented on every instructions, and with the time interrupt
	// delay in the wfi instruction. To change the clock to follow
	// real wall clock time, change the time to use the time since the
	// CPU start:
	//
	//	cpu.Time = uint64(time.Since(cpu.StartTime).Nanoseconds()) / 10
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
						log.Printf("CPU: trap %v\n", trap)
						if trap.Err != nil {
							log.Printf("  caused by %v\n", trap.Err)
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
	if cpu.MMU.MMIO != nil {
		return cpu.MMU.MMIO.Halt()
	}
	return nil
}

func (cpu *CPU) Shutdown() {
	cpu.shutdown = true
}

func (cpu *CPU) SetTrace(on bool) {
	// cpu.DebugTrace = on
}

func (cpu *CPU) loop() error {
	if cpu.PC%2 == 1 {
		return cpu.Trap(isa.CauseInstAddrMisaligned, cpu.PC, nil)
	}

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
		if cpu.Instret&0x3f == 0 { // || cpu.CSR[CsrMip] != 0 {
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
					bitMask := uint64(1 << bit)
					if pending&bitMask == 0 {
						continue
					}

					if mideleg&bitMask == 0 {
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

		if memory.Page(cpu.PC) != cpu.codePagenum {
			paddr, err := cpu.MMU.Map(cpu.PC, mmu.AccessExec)
			if err != nil {
				return err
			}
			cpu.codePage, err = cpu.MMU.Mem.Page(memory.Page(paddr))
			if err != nil {
				return cpu.Trap(isa.CauseInstAccessFault, paddr, err)
			}
			cpu.codePagenum = memory.Page(cpu.PC)
		}
		ofs := memory.PageOffset(cpu.PC)
		raw := uint32(cpu.codePage[ofs]) | uint32(cpu.codePage[ofs+1])<<8

		if raw&0b11 == 0b11 {
			// 32-bit instruction.
			if cpu.PC>>12 == (cpu.PC+2)>>12 {
				// Same page.
				raw |= uint32(cpu.codePage[ofs+2]) << 16
				raw |= uint32(cpu.codePage[ofs+3]) << 24
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
					cpu.tracef(raw, instr, "decode failed: %v", err)
					cpu.Dump(cpu.PC)
					return cpu.Trap(isa.CauseIllegalInstr, uint64(raw), err)
				}
				cpu.decodeCache[idx].Raw = raw
				cpu.decodeCache[idx].Instr = instr
			}
		} else {
			size = 2
			instr = isa.DecodeCFast(uint16(raw))
		}

		cpu.Instret++

		if cpuDebug || cpu.DebugTrace || cpu.DebugTrace2 {
			if cpu.Symtab != nil {
				mapped, entry := cpu.kernelMap(cpu.PC)
				if entry != nil && entry != cpu.LastSymbol {
					if cpu.DebugTrace {
						log.Printf("%v  <%s+0x%x>:\r\n",
							fmtAddr(cpu.PC), entry.Name, mapped-entry.Start)
					}
					cpu.LastSymbol = entry
				}
				if entry != nil {
					switch entry.Name {
					case "__delay":
						if false {
							// cpu.MMU.Mem.Strings()
							os.Exit(1)
						}

					case "mount_root_generic":
						cpu.DebugTrace = true
					}
				}
			}
			if cpuDebug || cpu.DebugTrace {
				cpu.trace(raw, instr, "")
			}
		}

		switch instr.Op {
		case isa.Invalid:
			cpu.tracef(raw, instr, "invalid: %v", instr)
			cpu.Dump(cpu.PC)
			err = fmt.Errorf("invalid %v[0x%x]", instr, raw)
			if true {
				return err
			} else {
				return cpu.Trap(isa.CauseIllegalInstr, uint64(raw), err)
			}

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
			if cpu.Trace {
				cpu.traceFunc(cpu.PC)
				cpu.tracef(raw, instr, "mode=%v => %v, sepc=%x",
					cpu.Mode(), cpu.mstatus.SPP(), cpu.CSR[CsrSepc])
			}
			cpu.mstatus.SetSIE(cpu.mstatus.SPIE())
			cpu.mstatus.SetSPIE(true)
			cpu.SetMode(cpu.mstatus.SPP())
			cpu.mstatus.SetSPP(isa.ModeU)
			cpu.PC = cpu.CSR[CsrSepc]
			cpu.ReservationValid = false
			continue

		case isa.Mret:
			if cpu.Trace {
				cpu.traceFunc(cpu.PC)
				cpu.tracef(raw, instr, "mode=%v => %v, mepc=%x",
					cpu.Mode(), cpu.mstatus.MPP(), cpu.CSR[CsrMepc])
			}
			cpu.mstatus.SetMIE(cpu.mstatus.MPIE())
			cpu.mstatus.SetMPIE(true)
			cpu.SetMode(cpu.mstatus.MPP())
			cpu.mstatus.SetMPP(isa.ModeU)
			cpu.PC = cpu.CSR[CsrMepc]
			cpu.ReservationValid = false
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
			now := cpu.syncTime()

			if stimecmp == 0xffffffffffffffff || now >= stimecmp {
				break
			}
			cpu.m.Lock()
			cpu.wfiTimeout = false
			cpu.m.Unlock()

			delay := stimecmp - now
			delayns := time.Duration(delay) * 10
			go func() {
				time.Sleep(delayns)
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
			if cpu.wfiTimeout {
				cpu.Time += delay
			}
			cpu.m.Unlock()

			// Check timer interrupts.
			if cpu.syncTime() >= cpu.CSR[CsrStimecmp] {
				cpu.CSR[CsrMip] |= isa.IntSTIP
			}

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
			f32 := math.Float32frombits(v32)
			cpu.F[instr.Rd] = float64(f32)

		case isa.Fsd:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v := math.Float64bits(cpu.F[instr.Rs2])
			if err := cpu.MMU.Store64(addr, v); err != nil {
				return err
			}
			cpu.ReservationValid = false

		case isa.Fsw:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			v := math.Float32bits(float32(cpu.F[instr.Rs2]))
			if err := cpu.MMU.Store32(addr, uint32(v)); err != nil {
				return err
			}
			cpu.ReservationValid = false

		case isa.Fence:

		case isa.SfenceVMA:
			cpu.MMU.FlushTLB()
			cpu.codePagenum = 0
			cpu.ReservationValid = false

		case isa.FeqS:
			if float32(cpu.F[instr.Rs1]) == float32(cpu.F[instr.Rs2]) {
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

		case isa.Mulhsu:
			rs1Val := cpu.X[instr.Rs1] // Treated as SIGNED
			rs2Val := cpu.X[instr.Rs2] // Treated as UNSIGNED

			// Perform standard unsigned 64x64 -> 128 multiplication.
			hi, _ := bits.Mul64(rs1Val, rs2Val)

			// If rs1 is negative when viewed as a signed 64-bit int,
			// apply the 2's complement high-word compensation.
			if int64(rs1Val) < 0 {
				hi -= rs2Val
			}

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
				cpu.X[instr.Rd] = uint64(int64(int32(uint32(cpu.X[instr.Rs1]))))
			} else {
				cpu.X[instr.Rd] = uint64(int64(int32(uint32(cpu.X[instr.Rs1]) %
					uint32(cpu.X[instr.Rs2]))))
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
			err = cpu.MMU.Store8(addr, uint8(cpu.X[instr.Rs2]))
			if err != nil {
				return err
			}
			cpu.ReservationValid = false

		case isa.Sd:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))

			// Direct TLB check.
			vpn := addr >> 12
			tlb := &cpu.MMU.TLB[vpn&0xfff]

			if cpu.mode <= isa.ModeS && tlb.VPN == vpn &&
				tlb.Flags.Writable() && tlb.Flags.Dirty() &&
				memory.Avail(addr, 8) {
				// Fast path: TLB hit.
				paddr := tlb.Page | (addr & uint64(tlb.OffsetMask))
				bo.PutUint64(cpu.MMU.Mem.RAM[cpu.MMU.Mem.Offset(paddr):],
					cpu.X[instr.Rs2])
			} else {
				// Slow path fallback.
				tlb.Clear()
				if err := cpu.MMU.Store64(addr, cpu.X[instr.Rs2]); err != nil {
					return err
				}
			}
			cpu.ReservationValid = false

		case isa.Sh:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			err = cpu.MMU.Store16(addr, uint16(cpu.X[instr.Rs2]))
			if err != nil {
				return err
			}
			cpu.ReservationValid = false

		case isa.Sw:
			addr := uint64(int64(cpu.X[instr.Rs1]) + int64(instr.Imm))
			err = cpu.MMU.Store32(addr, uint32(cpu.X[instr.Rs2]))
			if err != nil {
				return err
			}
			cpu.ReservationValid = false

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
				(cpu.X[instr.Rs2] & 0x3f))

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
			cpu.X[instr.Rd] = uint64(int64(int32(uint32(cpu.X[instr.Rs1]) >>
				(cpu.X[instr.Rs2] & 0b11111))))

		case isa.Srli:
			cpu.X[instr.Rd] = cpu.X[instr.Rs1] >> instr.Imm

		case isa.Srliw:
			cpu.X[instr.Rd] = uint64(int64(int32(uint32(cpu.X[instr.Rs1]) >>
				instr.Imm)))

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
			err = cpu.MMU.Store32(addr, uint32(cpu.X[instr.Rs2]))
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
			t := uint32(int32(v) + int32(cpu.X[instr.Rs2]))
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
			t := uint32(int32(v) & int32(cpu.X[instr.Rs2]))
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
			t := uint32(int32(v) | int32(cpu.X[instr.Rs2]))
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
			t := uint32(int32(v) ^ int32(cpu.X[instr.Rs2]))
			err = cpu.MMU.Store32(addr, t)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int32(v)))

		case isa.AmomaxD:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.MMU.Load64(addr)
			if err != nil {
				return err
			}
			t := cpu.X[instr.Rs2]
			if int64(v) > int64(t) {
				t = v
			}
			err = cpu.MMU.Store64(addr, t)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = v

		case isa.AmomaxW:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.MMU.Load32(addr)
			if err != nil {
				return err
			}
			t := uint32(cpu.X[instr.Rs2])
			if int32(v) > int32(t) {
				t = uint32(v)
			}
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
			err = cpu.MMU.Store32(addr, t)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int32(v)))

		case isa.AmominD:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.MMU.Load64(addr)
			if err != nil {
				return err
			}
			t := cpu.X[instr.Rs2]
			if int64(v) < int64(t) {
				t = v
			}
			err = cpu.MMU.Store64(addr, t)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = v

		case isa.AmominW:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.MMU.Load32(addr)
			if err != nil {
				return err
			}
			t := uint32(cpu.X[instr.Rs2])
			if int32(v) < int32(t) {
				t = uint32(v)
			}
			err = cpu.MMU.Store32(addr, t)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int32(v)))

		case isa.AmominuD:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.MMU.Load64(addr)
			if err != nil {
				return err
			}
			t := cpu.X[instr.Rs2]
			if v < t {
				t = v
			}
			err = cpu.MMU.Store64(addr, t)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = v

		case isa.AmominuW:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.MMU.Load32(addr)
			if err != nil {
				return err
			}
			t := uint32(cpu.X[instr.Rs2])
			if uint32(v) < t {
				t = uint32(v)
			}
			err = cpu.MMU.Store32(addr, t)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int32(v)))

		case isa.LrW:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.MMU.Load32(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = uint64(int64(int32(v)))

			// Register the reservation.
			cpu.Reservation = addr
			cpu.ReservationValid = true

		case isa.LrD:
			addr := cpu.X[instr.Rs1]
			v, err := cpu.MMU.Load64(addr)
			if err != nil {
				return err
			}
			cpu.X[instr.Rd] = v

			// Register the reservation
			cpu.Reservation = addr
			cpu.ReservationValid = true

		case isa.ScW:
			addr := cpu.X[instr.Rs1]

			// SC succeeds only if the reservation matches.
			if cpu.ReservationValid && cpu.Reservation == addr {
				err := cpu.MMU.Store32(addr, uint32(cpu.X[instr.Rs2]))
				if err != nil {
					cpu.ReservationValid = false
					return err
				}
				cpu.X[instr.Rd] = 0 // 0 = Success
			} else {
				cpu.X[instr.Rd] = 1 // 1 = Failure
			}
			cpu.ReservationValid = false

		case isa.ScD:
			addr := cpu.X[instr.Rs1]

			if cpu.ReservationValid && cpu.Reservation == addr {
				err := cpu.MMU.Store64(addr, cpu.X[instr.Rs2])
				if err != nil {
					cpu.ReservationValid = false
					return err
				}
				cpu.X[instr.Rd] = 0
			} else {
				cpu.X[instr.Rd] = 1
			}
			cpu.ReservationValid = false

			// Floating point extension.

		case isa.FaddD:
			cpu.F[instr.Rd] = cpu.F[instr.Rs1] + cpu.F[instr.Rs2]

		case isa.FaddS:
			cpu.F[instr.Rd] = float64(float32(cpu.F[instr.Rs1]) +
				float32(cpu.F[instr.Rs2]))

		case isa.FsubD:
			cpu.F[instr.Rd] = cpu.F[instr.Rs1] - cpu.F[instr.Rs2]

		case isa.FsubS:
			cpu.F[instr.Rd] = float64(float32(cpu.F[instr.Rs1]) -
				float32(cpu.F[instr.Rs2]))

		case isa.FmulD:
			cpu.F[instr.Rd] = cpu.F[instr.Rs1] * cpu.F[instr.Rs2]

		case isa.FmulS:
			cpu.F[instr.Rd] = float64(float32(cpu.F[instr.Rs1]) *
				float32(cpu.F[instr.Rs2]))

		case isa.FdivD:
			cpu.F[instr.Rd] = cpu.F[instr.Rs1] / cpu.F[instr.Rs2]

		case isa.FdivS:
			cpu.F[instr.Rd] = float64(float32(cpu.F[instr.Rs1]) /
				float32(cpu.F[instr.Rs2]))

		case isa.FmsubD:
			cpu.F[instr.Rd] = cpu.F[instr.Rs1]*cpu.F[instr.Rs2] -
				cpu.F[instr.Imm]

		case isa.FnmsubD:
			cpu.F[instr.Rd] = -(cpu.F[instr.Rs1] * cpu.F[instr.Rs2]) +
				cpu.F[instr.Imm]

		case isa.FnmaddD:
			cpu.F[instr.Rd] = -(cpu.F[instr.Rs1] * cpu.F[instr.Rs2]) -
				cpu.F[instr.Imm]

		case isa.FsqrtD:
			cpu.F[instr.Rd] = math.Sqrt(cpu.F[instr.Rs1])

		case isa.FleD:
			if cpu.F[instr.Rs1] <= cpu.F[instr.Rs2] {
				cpu.X[instr.Rd] = 1
			} else {
				cpu.X[instr.Rd] = 0
			}

		case isa.FltD:
			if cpu.F[instr.Rs1] < cpu.F[instr.Rs2] {
				cpu.X[instr.Rd] = 1
			} else {
				cpu.X[instr.Rd] = 0
			}

		case isa.FeqD:
			if cpu.F[instr.Rs1] == cpu.F[instr.Rs2] {
				cpu.X[instr.Rd] = 1
			} else {
				cpu.X[instr.Rd] = 0
			}

		case isa.FmvDX:
			cpu.F[instr.Rd] = math.Float64frombits(cpu.X[instr.Rs1])

		case isa.FmvWX:
			v := uint32(cpu.X[instr.Rs1])
			cpu.F[instr.Rd] = float64(math.Float32frombits(v))

		case isa.FmvXD:
			cpu.X[instr.Rd] = math.Float64bits(cpu.F[instr.Rs1])

		case isa.FmvXW:
			cpu.X[instr.Rd] = uint64(int64(int32(
				math.Float32bits(float32(cpu.F[instr.Rs1])))))

		case isa.FclassD:
			cpu.X[instr.Rd] = fclassD(cpu.F[instr.Rs1])

		case isa.FclassS:
			cpu.X[instr.Rd] = uint64(fclassS(float32(cpu.F[instr.Rs1])))

		case isa.FmaddS:
			// Imm is Rs3
			cpu.F[instr.Rd] = float64(float32(cpu.F[instr.Rs1])*
				float32(cpu.F[instr.Rs2]) + float32(cpu.F[instr.Imm]))

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

		case isa.FcvtDWU:
			cpu.F[instr.Rd] = float64(uint32(cpu.X[instr.Rs1]))

		case isa.FcvtLD:
			// XXX If the value is out of range, fcsr.fflags.NV is set
			// to 1
			f := cpu.F[instr.Rs1]
			var v uint64
			if math.IsNaN(f) {
				// RISC-V default NaN value for signed 64-bit
				v = 0x7fffffffffffffff
			} else if f >= math.MaxInt64 {
				// Handles +Inf and positive overflow.
				v = 0x7fffffffffffffff
			} else if f < math.MinInt64 {
				// Handles -Inf and negative overflow.
				v = 0x8000000000000000
			} else {
				v = uint64(int64(f))
			}
			cpu.X[instr.Rd] = v

		case isa.FcvtLUD:
			// XXX If the value is out of range, fcsr.fflags.NV is set
			// to 1
			f := cpu.F[instr.Rs1]
			var v uint64
			if math.IsNaN(f) {
				// RISC-V default NaN value for signed 64-bit
				v = 0xffffffffffffffff
			} else if f >= math.MaxUint64 {
				// Handles +Inf and positive overflow.
				v = 0xffffffffffffffff
			} else if f < 0.0 {
				// Handles -Inf and negative overflow.
				v = 0
			} else {
				v = uint64(f)
			}
			cpu.X[instr.Rd] = v

		case isa.FcvtWD:
			cpu.X[instr.Rd] = uint64(int64(int32(cpu.F[instr.Rs1])))

		case isa.FcvtWUD:
			cpu.X[instr.Rd] = uint64(uint32(cpu.F[instr.Rs1]))

		case isa.FcvtLUS:
			cpu.X[instr.Rd] = uint64(cpu.F[instr.Rs1])

		case isa.FcvtSD:
			cpu.F[instr.Rd] = float64(float32(cpu.F[instr.Rs1]))

		case isa.FcvtSW:
			cpu.F[instr.Rd] = float64(float32(int32(cpu.X[instr.Rs1])))

		case isa.FcvtSWU:
			cpu.F[instr.Rd] = float64(float32(uint32(cpu.X[instr.Rs1])))

		case isa.FcvtSL:
			cpu.F[instr.Rd] = float64(float32(int64(cpu.X[instr.Rs1])))

		case isa.FcvtSLU:
			cpu.F[instr.Rd] = float64(float32(cpu.X[instr.Rs1]))

		case isa.FcvtDS:
			cpu.F[instr.Rd] = cpu.F[instr.Rs1]

		case isa.FcvtDW:
			cpu.F[instr.Rd] = float64(int32(cpu.X[instr.Rs1]))

		case isa.FsgnjD:
			v := math.Float64bits(cpu.F[instr.Rs1])
			b := math.Float64bits(cpu.F[instr.Rs2])

			v &^= 1 << 63
			v |= b & (1 << 63)

			cpu.F[instr.Rd] = math.Float64frombits(v)

		case isa.FsgnjnD:
			v := math.Float64bits(cpu.F[instr.Rs1])
			b := math.Float64bits(cpu.F[instr.Rs2])

			v &^= 1 << 63
			v |= (^b) & (1 << 63) // Inject the inverted sign bit

			cpu.F[instr.Rd] = math.Float64frombits(v)

		case isa.FsgnjxD:
			v := math.Float64bits(cpu.F[instr.Rs1])
			b := math.Float64bits(cpu.F[instr.Rs2])

			vs := v & (1 << 63)
			bs := b & (1 << 63)

			v &^= 1 << 63
			v |= vs ^ bs // XOR the sign bits

			cpu.F[instr.Rd] = math.Float64frombits(v)

		case isa.FsgnjS:
			v := math.Float32bits(float32(cpu.F[instr.Rs1]))
			b := math.Float32bits(float32(cpu.F[instr.Rs2]))

			v &^= 1 << 31
			v |= b & (1 << 31)

			cpu.F[instr.Rd] = float64(math.Float32frombits(v))

		case isa.FsgnjnS:
			v := math.Float32bits(float32(cpu.F[instr.Rs1]))
			b := math.Float32bits(float32(cpu.F[instr.Rs2]))

			v &^= 1 << 31
			v |= (^b) & (1 << 31) // Inject the inverted sign bit

			cpu.F[instr.Rd] = float64(math.Float32frombits(v))

		case isa.FsgnjxS:
			v := math.Float32bits(float32(cpu.F[instr.Rs1]))
			b := math.Float32bits(float32(cpu.F[instr.Rs2]))

			vs := v & (1 << 31)
			bs := b & (1 << 31)

			v &^= 1 << 31
			v |= vs ^ bs

			cpu.F[instr.Rd] = float64(math.Float32frombits(v))

		case isa.FminD:
			if cpu.F[instr.Rs1] < cpu.F[instr.Rs2] {
				cpu.F[instr.Rd] = cpu.F[instr.Rs1]
			} else {
				cpu.F[instr.Rd] = cpu.F[instr.Rs2]
			}

		case isa.FminS:
			if float32(cpu.F[instr.Rs1]) < float32(cpu.F[instr.Rs2]) {
				cpu.F[instr.Rd] = cpu.F[instr.Rs1]
			} else {
				cpu.F[instr.Rd] = cpu.F[instr.Rs2]
			}

		case isa.FmaxD:
			if cpu.F[instr.Rs1] > cpu.F[instr.Rs2] {
				cpu.F[instr.Rd] = cpu.F[instr.Rs1]
			} else {
				cpu.F[instr.Rd] = cpu.F[instr.Rs2]
			}

		case isa.FmaxS:
			if float32(cpu.F[instr.Rs1]) > float32(cpu.F[instr.Rs2]) {
				cpu.F[instr.Rd] = cpu.F[instr.Rs1]
			} else {
				cpu.F[instr.Rd] = cpu.F[instr.Rs2]
			}

			// Extension 'B' Bit Manipulation.

		case isa.Maxu:
			if cpu.X[instr.Rs1] > cpu.X[instr.Rs2] {
				cpu.X[instr.Rd] = cpu.X[instr.Rs1]
			} else {
				cpu.X[instr.Rd] = cpu.X[instr.Rs2]
			}

			// Zicond Extension (Conditional Integer Operations).

		case isa.CzeroEqz:
			if cpu.X[instr.Rs2] == 0 {
				cpu.X[instr.Rd] = 0
			} else {
				cpu.X[instr.Rd] = cpu.X[instr.Rs1]
			}

		case isa.CzeroNez:
			if cpu.X[instr.Rs2] != 0 {
				cpu.X[instr.Rd] = 0
			} else {
				cpu.X[instr.Rd] = cpu.X[instr.Rs1]
			}

			// Zba (Address Generation Instructions) extensions.

		case isa.AddUw:
			cpu.X[instr.Rd] = uint64(uint32(cpu.X[instr.Rs1])) +
				cpu.X[instr.Rs2]

		case isa.Sh1add:
			cpu.X[instr.Rd] = cpu.X[instr.Rs2] + (cpu.X[instr.Rs1] << 1)

		case isa.Sh2add:
			cpu.X[instr.Rd] = cpu.X[instr.Rs2] + (cpu.X[instr.Rs1] << 2)

		case isa.Sh3add:
			cpu.X[instr.Rd] = cpu.X[instr.Rs2] + (cpu.X[instr.Rs1] << 3)

		case isa.Sh1addUw:
			cpu.X[instr.Rd] = cpu.X[instr.Rs2] +
				(uint64(uint32(cpu.X[instr.Rs1])) << 1)

		case isa.Sh2addUw:
			cpu.X[instr.Rd] = cpu.X[instr.Rs2] +
				(uint64(uint32(cpu.X[instr.Rs1])) << 2)

		case isa.Sh3addUw:
			cpu.X[instr.Rd] = cpu.X[instr.Rs2] +
				(uint64(uint32(cpu.X[instr.Rs1])) << 3)

			// Vector extension.

			// Load and store instructions:
			//
			// 	 0:1 vm - 1 unmasked, 0 masked
			// 	 1:3 mop:
			// 	     - 000 unit-stride
			// 	     - 010 strided
			// 	     - 011 indexed (unordered)
			// 	     - 111 indexed (ordered)
			// 	 4:6 nf - number of fields = nf+1

		case isa.Vsetvli:
			if cpu.mstatus.VS() == isa.RegOff {
				return cpu.Trap(isa.CauseIllegalInstr, 0, nil)
			}
			vtype := isa.VType(instr.Imm)
			cpu.vpu.VType = vtype
			maxVL := uint64(float32(cpu.vpu.VLEN)*vtype.VLMUL()) /
				uint64(vtype.VSEW())

			requestedVL := cpu.X[instr.Rs1]
			if requestedVL > maxVL {
				cpu.vpu.VL = maxVL
			} else {
				cpu.vpu.VL = requestedVL
			}
			cpu.X[instr.Rd] = cpu.vpu.VL
			cpu.vpu.VStart = 0
			cpu.mstatus.SetVS(isa.RegDirty)

		case isa.Vsetivli:
			if cpu.mstatus.VS() == isa.RegOff {
				return cpu.Trap(isa.CauseIllegalInstr, 0, nil)
			}
			vtype := isa.VType(instr.Imm)
			cpu.vpu.VType = vtype
			maxVL := uint64(float32(cpu.vpu.VLEN)*vtype.VLMUL()) /
				uint64(vtype.VSEW())

			var requestedVL uint64
			if instr.Rs1 == 0 {
				requestedVL = maxVL
			} else {
				requestedVL = cpu.X[instr.Rs1]
			}

			if requestedVL > maxVL {
				cpu.vpu.VL = maxVL
			} else {
				cpu.vpu.VL = requestedVL
			}
			cpu.X[instr.Rd] = cpu.vpu.VL
			cpu.vpu.VStart = 0
			cpu.mstatus.SetVS(isa.RegDirty)

		case isa.VmvVX:
			if cpu.mstatus.VS() == isa.RegOff {
				return cpu.Trap(isa.CauseIllegalInstr, 0, nil)
			}
			vl := cpu.vpu.VL
			sew := cpu.vpu.VType.VSEW()
			scalarVal := cpu.X[instr.Rs1]
			dest := cpu.vpu.VRegs[instr.Rd]

			switch sew {
			case 8:
				val8 := uint8(scalarVal)
				for i := uint64(0); i < vl; i++ {
					dest[i] = val8
				}

			case 16:
				val16 := uint16(scalarVal)
				for i := uint64(0); i < vl; i++ {
					bo.PutUint16(dest[i*2:], val16)
				}

			case 32:
				val32 := uint32(scalarVal)
				for i := uint64(0); i < vl; i++ {
					bo.PutUint32(dest[i*4:], val32)
				}

			case 64:
				for i := uint64(0); i < vl; i++ {
					bo.PutUint64(dest[i*8:], scalarVal)
				}
			}
			cpu.vpu.VStart = 0
			cpu.mstatus.SetVS(isa.RegDirty)

		case isa.VmvVI:
			if cpu.mstatus.VS() == isa.RegOff {
				return cpu.Trap(isa.CauseIllegalInstr, 0, nil)
			}
			vl := cpu.vpu.VL
			sew := cpu.vpu.VType.VSEW()
			dest := cpu.vpu.VRegs[instr.Rd]

			switch sew {
			case 8:
				val8 := uint8(instr.Imm)
				for i := cpu.vpu.VStart; i < vl; i++ {
					dest[i] = val8
				}

			case 16:
				val16 := uint16(instr.Imm)
				for i := cpu.vpu.VStart; i < vl; i++ {
					bo.PutUint16(dest[i*2:], val16)
				}

			case 32:
				val32 := uint32(instr.Imm)
				for i := cpu.vpu.VStart; i < vl; i++ {
					bo.PutUint32(dest[i*4:], val32)
				}

			case 64:
				val64 := uint64(instr.Imm)
				for i := cpu.vpu.VStart; i < vl; i++ {
					bo.PutUint64(dest[i*8:], val64)
				}
			}
			cpu.vpu.VStart = 0
			cpu.mstatus.SetVS(isa.RegDirty)

		case isa.Vle8V:
			if cpu.mstatus.VS() == isa.RegOff {
				return cpu.Trap(isa.CauseIllegalInstr, 0, nil)
			}
			vm := instr.Imm & 0b1
			mop := instr.Imm >> 1 & 0b111
			nf := instr.Imm >> 4 & 0b111

			if vm != 1 || mop != 0 || nf != 0 {
				return cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
					fmt.Errorf("instruction %v not implemented yet", instr))
			}

			baseAddr := cpu.X[instr.Rs1]
			vl := cpu.vpu.VL
			dstVec := cpu.vpu.VRegs[instr.Rd]

			for i := cpu.vpu.VStart; i < vl; i++ {
				srcAddr := baseAddr + i
				val, err := cpu.MMU.Load8(srcAddr)
				if err != nil {
					cpu.vpu.VStart = i
					return err
				}
				dstVec[i] = val
			}
			cpu.vpu.VStart = 0
			cpu.mstatus.SetVS(isa.RegDirty)

		case isa.Vse8V:
			if cpu.mstatus.VS() == isa.RegOff {
				return cpu.Trap(isa.CauseIllegalInstr, 0, nil)
			}
			vm := instr.Imm & 0b1
			mop := instr.Imm >> 1 & 0b111
			nf := instr.Imm >> 4 & 0b111

			if vm != 1 || mop != 0 || nf != 0 {
				return cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
					fmt.Errorf("instruction %v not implemented yet", instr))
			}

			baseAddr := cpu.X[instr.Rs1]
			vl := cpu.vpu.VL
			srcVec := cpu.vpu.VRegs[instr.Rd]

			for i := cpu.vpu.VStart; i < vl; i++ {
				if i+1 > uint64(len(srcVec)) {
					return cpu.Trap(isa.CauseIllegalInstr, uint64(raw), nil)
				}
				v := srcVec[i]

				targetAddr := baseAddr + i
				err := cpu.MMU.Store8(targetAddr, v)
				if err != nil {
					cpu.vpu.VStart = i
					return err
				}
			}
			cpu.vpu.VStart = 0
			cpu.mstatus.SetVS(isa.RegDirty)

		case isa.Vse64V:
			if cpu.mstatus.VS() == isa.RegOff {
				return cpu.Trap(isa.CauseIllegalInstr, 0, nil)
			}
			vm := instr.Imm & 0b1
			mop := instr.Imm >> 1 & 0b111
			nf := instr.Imm >> 4 & 0b111

			if vm != 1 || mop != 0 || nf != 0 {
				return cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
					fmt.Errorf("instruction %v not implemented yet", instr))
			}

			baseAddr := cpu.X[instr.Rs1]
			vl := cpu.vpu.VL
			srcVec := cpu.vpu.VRegs[instr.Rd]

			for i := cpu.vpu.VStart; i < vl; i++ {
				elementOfs := i * 8
				if elementOfs+8 > uint64(len(srcVec)) {
					return cpu.Trap(isa.CauseIllegalInstr, uint64(raw), nil)
				}
				v := bo.Uint64(srcVec[elementOfs:])

				targetAddr := baseAddr + i*8
				err := cpu.MMU.Store64(targetAddr, v)
				if err != nil {
					cpu.vpu.VStart = i
					return err
				}
			}
			cpu.vpu.VStart = 0
			cpu.mstatus.SetVS(isa.RegDirty)

		default:
			cpu.tracef(raw, instr, "not implemented")
			cpu.Dump(cpu.PC)
			return cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("instruction %v[0x%x] not implemented yet",
					instr, raw))
		}
		cpu.PC += uint64(size)
	}
}

func fclassD(fVal float64) uint64 {
	bits := math.Float64bits(fVal)

	sign := (bits >> 63) == 1
	exponent := (bits >> 52) & 0x7FF
	mantissa := bits & 0xFFFFFFFFFFFFF

	var mask uint64

	if exponent == 0x7FF {
		if mantissa == 0 {
			if sign {
				mask = 1 << 0 // -Infinity
			} else {
				mask = 1 << 7 // +Infinity
			}
		} else {
			// NaN check (Signal vs Quiet)
			// RISC-V typically uses the MSB of the mantissa to denote
			// Quiet NaN (1) vs Signaling NaN (0)
			isQuiet := (mantissa & (1 << 51)) != 0
			if !isQuiet {
				mask = 1 << 8 // Signaling NaN
			} else {
				mask = 1 << 9 // Quiet NaN
			}
		}
	} else if exponent == 0 {
		if mantissa == 0 {
			if sign {
				mask = 1 << 3 // -0
			} else {
				mask = 1 << 4 // +0
			}
		} else {
			if sign {
				mask = 1 << 2 // Negative subnormal
			} else {
				mask = 1 << 5 // Positive subnormal
			}
		}
	} else {
		if sign {
			mask = 1 << 1 // Negative normal
		} else {
			mask = 1 << 6 // Positive normal
		}
	}

	return mask
}

func fclassS(fVal float32) uint32 {
	bits := math.Float32bits(fVal)

	sign := (bits >> 31) == 1
	exponent := (bits >> 23) & 0xFF
	mantissa := bits & 0x7FFFFF

	var mask uint32

	if exponent == 0xFF {
		if mantissa == 0 {
			if sign {
				mask = 1 << 0 // -Infinity
			} else {
				mask = 1 << 7 // +Infinity
			}
		} else {
			// NaN check (Signal vs Quiet)
			// RISC-V uses the MSB of the mantissa (bit 22 for float32)
			isQuiet := (mantissa & (1 << 22)) != 0
			if !isQuiet {
				mask = 1 << 8 // Signaling NaN
			} else {
				mask = 1 << 9 // Quiet NaN
			}
		}
	} else if exponent == 0 {
		if mantissa == 0 {
			if sign {
				mask = 1 << 3 // -0
			} else {
				mask = 1 << 4 // +0
			}
		} else {
			if sign {
				mask = 1 << 2 // Negative subnormal
			} else {
				mask = 1 << 5 // Positive subnormal
			}
		}
	} else {
		if sign {
			mask = 1 << 1 // Negative normal
		} else {
			mask = 1 << 6 // Positive normal
		}
	}

	return mask
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
		log.Printf(" - mapped=%x\n", mapped)
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

	log.Printf("%s%8x  <%s+0x%x>:%s",
		cpu.ColorOn(),
		pc, entry.Name, mapped-entry.Start,
		cpu.ColorOff())

	return entry
}

func (cpu *CPU) tracef(raw uint32, instr isa.Instr,
	format string, args ...interface{}) {

	cpu.trace(raw, instr, fmt.Sprintf(format, args...))
}

func (cpu *CPU) ColorOn() string {
	if !cpuColor {
		return ""
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
	return fmt.Sprintf("\x1b[%sm", color)
}

func (cpu *CPU) ColorOff() string {
	if !cpuColor {
		return ""
	}
	return "\x1b[0m"
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
	var line string

	addr := fmtAddr(cpu.PC)
	if raw&0b11 == 0b11 {
		line = fmt.Sprintf("%s:  %08x   %v", addr, raw, instr)
	} else {
		line = fmt.Sprintf("%s:  %04x       %v", addr, raw, instr)
	}
	if len(msg) == 0 {
		if true {
			switch instr.Op {
			case isa.Auipc:
				msg = fmt.Sprintf("pc=%x, imm=%x", cpu.PC, instr.Imm)

			case isa.Addi:
				msg = fmt.Sprintf("%v=%x, imm=%x",
					instr.Rs1, cpu.X[instr.Rs1], instr.Imm)

			case isa.Jal:
				msg = fmt.Sprintf("imm=%x", instr.Imm)

			default:
				if instr.Rs1 != 0 {
					msg = fmt.Sprintf("%v=%x", instr.Rs1, cpu.X[instr.Rs1])
				}
				if instr.Rs2 != 0 {
					if len(msg) > 0 {
						msg += ","
					}
					msg += fmt.Sprintf("%v=%x", instr.Rs2, cpu.X[instr.Rs2])
				}
			}
		} else {
			op, ok := isa.Operands[instr.Op]
			if ok && len(op.Desc) > 0 && instr.Op != cpu.lastDescOp {
				cpu.lastDescOp = instr.Op
				msg = op.Desc
			}
		}
	}
	if len(msg) > 0 {
		for len(line) < 46 {
			line += " "
		}
		line += fmt.Sprintf(" # %s", msg)
	}
	log.Printf("%s%s%s", cpu.ColorOn(), line, cpu.ColorOff())
}
