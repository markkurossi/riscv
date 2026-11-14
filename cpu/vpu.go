//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package cpu

import (
	"fmt"
	"math"
	"math/bits"

	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/memory"
)

const (
	// ELEN defines the maximum size in bits of a single vector
	// element that any operation can produce or consume.
	ELEN = 64

	// Physical width of each register in bits (e.g., 128, 256, 512).
	VLEN uint64 = 128

	vpuDebug = false
)

// VPU implements the Vector Processing Unit.
type VPU struct {
	cpu *CPU

	// 32 Vector Registers. Each register can hold up to VLEN bits.
	// Stored as a flat byte slice per register for easy, dynamic
	// element casting.
	VRegs [32][]byte

	// Vector Control and Status Registers (VCSRs)
	VType  isa.VType // Tracks VSEW, VLMUL, VTA, VMA
	VL     uint64    // Active vector length (dynamic element count)
	VStart uint64    // Elements processing start index (for traps/resumes)
	VXRM   uint8     // Fixed-point rounding mode
	VXSat  bool      // Fixed-point saturation flag

	// The maximum VL value with the current vector configuration.
	VLMax uint64
}

// NewVPU creates a new Vector Processing Unit for the CPU.
func NewVPU(cpu *CPU) *VPU {
	vpu := &VPU{
		cpu: cpu,
	}
	for i := range vpu.VRegs {
		vpu.VRegs[i] = make([]byte, VLEN/8)
	}
	return vpu
}

func (vpu *VPU) execute(instr isa.Instr, raw uint32) error {
	if vpuDebug && vpu.cpu.DebugInstrTrace {
		vpu.cpu.tracef(raw, instr, "")
	}
	if vpu.cpu.mstatus.VS() == isa.RegOff {
		return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
			fmt.Errorf("VS=%v", vpu.cpu.mstatus.VS()))
	}
	vpu.cpu.mstatus.SetVS(isa.RegDirty)
	vpu.cpu.mstatus.SetSD(true)

	op, flags, imm := instr.VPU()

	// The flags are hardcoded in decode:
	vm := flags & 0b0001
	nf := uint64(flags>>1&0b111) + 1

	switch op {
	case isa.Vsetvl:
		vpu.setVL(instr.Rd, instr.Rs1, vpu.cpu.X[instr.Rs1],
			isa.VType(vpu.cpu.X[instr.Rs2]))

	case isa.Vsetvli:
		vpu.setVL(instr.Rd, instr.Rs1, vpu.cpu.X[instr.Rs1], isa.VType(imm))

	case isa.Vsetivli:
		vpu.setVL(instr.Rd, 0xff, uint64(instr.Rs1), isa.VType(imm))

	case isa.VmvVV:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)

			switch sew {
			case isa.E8:
				reg[ofs] = vs1reg[vs1ofs]

			case isa.E16:
				memory.PutUint16(reg, ofs, memory.Uint16(vs1reg, vs1ofs))

			case isa.E32:
				memory.PutUint32(reg, ofs, memory.Uint32(vs1reg, vs1ofs))

			case isa.E64:
				memory.PutUint64(reg, ofs, memory.Uint64(vs1reg, vs1ofs))
			}
		}

	case isa.VmergeVvm:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			var vsreg []byte
			var vsofs uint64

			if vpu.maskElt(0, i) {
				vsreg, vsofs = vpu.elt(sew, uint64(instr.Rs1), i)
			} else {
				vsreg, vsofs = vpu.elt(sew, uint64(instr.Rs2), i)
			}

			switch sew {
			case isa.E8:
				reg[ofs] = vsreg[vsofs]

			case isa.E16:
				memory.PutUint16(reg, ofs, memory.Uint16(vsreg, vsofs))

			case isa.E32:
				memory.PutUint32(reg, ofs, memory.Uint32(vsreg, vsofs))

			case isa.E64:
				memory.PutUint64(reg, ofs, memory.Uint64(vsreg, vsofs))
			}
		}

	case isa.VmergeVim:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)

			if vpu.maskElt(0, i) {
				switch sew {
				case isa.E8:
					reg[ofs] = uint8(int8(imm))
				case isa.E16:
					memory.PutUint16(reg, ofs, uint16(int16(imm)))
				case isa.E32:
					memory.PutUint32(reg, ofs, uint32(int32(imm)))
				case isa.E64:
					memory.PutUint64(reg, ofs, uint64(int64(imm)))
				}
			} else {
				vsreg, vsofs := vpu.elt(sew, uint64(instr.Rs2), i)
				switch sew {
				case isa.E8:
					reg[ofs] = vsreg[vsofs]
				case isa.E16:
					memory.PutUint16(reg, ofs, memory.Uint16(vsreg, vsofs))
				case isa.E32:
					memory.PutUint32(reg, ofs, memory.Uint32(vsreg, vsofs))
				case isa.E64:
					memory.PutUint64(reg, ofs, memory.Uint64(vsreg, vsofs))
				}
			}
		}

	case isa.VandVx:
		sew := vpu.VType.VSEW()
		rs1 := vpu.cpu.X[instr.Rs1]

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				v := vs2reg[vs2ofs]
				reg[ofs] = v & uint8(rs1)

			case isa.E16:
				v := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, v&uint16(rs1))

			case isa.E32:
				v := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, v&uint32(rs1))

			case isa.E64:
				v := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, v&uint64(rs1))
			}
		}

	case isa.VsllVx:
		sew := vpu.VType.VSEW()
		rs1 := vpu.cpu.X[instr.Rs1] & (uint64(sew) - 1)

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				v := vs2reg[vs2ofs]
				reg[ofs] = v << rs1

			case isa.E16:
				v := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, v<<rs1)

			case isa.E32:
				v := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, v<<rs1)

			case isa.E64:
				v := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, v<<rs1)
			}
		}

	case isa.VsrlVx:
		sew := vpu.VType.VSEW()
		rs1 := vpu.cpu.X[instr.Rs1] & (uint64(sew) - 1)

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				v := vs2reg[vs2ofs]
				reg[ofs] = v >> rs1

			case isa.E16:
				v := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, v>>rs1)

			case isa.E32:
				v := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, v>>rs1)

			case isa.E64:
				v := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, v>>rs1)
			}
		}

	case isa.VsraVx:
		sew := vpu.VType.VSEW()
		rs1 := vpu.cpu.X[instr.Rs1] & (uint64(sew) - 1)

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				v := vs2reg[vs2ofs]
				reg[ofs] = uint8(int8(v) >> rs1)

			case isa.E16:
				v := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, uint16(int16(v)>>rs1))

			case isa.E32:
				v := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, uint32(int32(v)>>rs1))

			case isa.E64:
				v := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, uint64(int64(v)>>rs1))
			}
		}

	case isa.VmulVx:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			rs1 := vpu.cpu.X[instr.Rs1]
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				v := vs2reg[vs2ofs]
				reg[ofs] = v * uint8(rs1)

			case isa.E16:
				v := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, v*uint16(rs1))

			case isa.E32:
				v := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, v*uint32(rs1))

			case isa.E64:
				v := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, v*uint64(rs1))
			}
		}

	case isa.VslidedownVx:
		sew := vpu.VType.VSEW()
		sh := vpu.cpu.X[instr.Rs1]

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			var offset uint64
			var isValid bool

			// sh can be 0xffffffffffffffff so sh+1 wraps around and
			// is smaller than vlmax. Instead of using uint128
			// variables, we check the condition with two checks:
			if sh < vpu.VLMax && i+sh < vpu.VLMax {
				isValid = true
			}
			if isValid {
				offset = sh
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i+offset)

			switch sew {
			case isa.E8:
				var v uint8
				if isValid {
					v = vs2reg[vs2ofs]
				}
				reg[ofs] = v

			case isa.E16:
				var v uint16
				if isValid {
					v = memory.Uint16(vs2reg, vs2ofs)
				}
				memory.PutUint16(reg, ofs, v)

			case isa.E32:
				var v uint32
				if isValid {
					v = memory.Uint32(vs2reg, vs2ofs)
				}
				memory.PutUint32(reg, ofs, v)

			case isa.E64:
				var v uint64
				if isValid {
					v = memory.Uint64(vs2reg, vs2ofs)
				}
				memory.PutUint64(reg, ofs, v)
			}
		}

	case isa.Vslide1downVx:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)

			if i != vpu.VL-1 {
				vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i+1)

				switch sew {
				case isa.E8:
					reg[ofs] = vs2reg[vs2ofs]
				case isa.E16:
					memory.PutUint16(reg, ofs, memory.Uint16(vs2reg, vs2ofs))
				case isa.E32:
					memory.PutUint32(reg, ofs, memory.Uint32(vs2reg, vs2ofs))
				case isa.E64:
					memory.PutUint64(reg, ofs, memory.Uint64(vs2reg, vs2ofs))
				}
			} else {
				rs1 := vpu.cpu.X[instr.Rs1]

				switch sew {
				case isa.E8:
					reg[ofs] = uint8(rs1)
				case isa.E16:
					memory.PutUint16(reg, ofs, uint16(rs1))
				case isa.E32:
					memory.PutUint32(reg, ofs, uint32(rs1))
				case isa.E64:
					memory.PutUint64(reg, ofs, uint64(rs1))
				}
			}
		}

	case isa.VaddVx:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			rs1 := vpu.cpu.X[instr.Rs1]
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				v := vs2reg[vs2ofs]
				reg[ofs] = v + uint8(rs1)

			case isa.E16:
				v := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, v+uint16(rs1))

			case isa.E32:
				v := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, v+uint32(rs1))

			case isa.E64:
				v := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, v+uint64(rs1))
			}
		}

	case isa.VsubVx:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			rs1 := vpu.cpu.X[instr.Rs1]
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				v := vs2reg[vs2ofs]
				reg[ofs] = v - uint8(rs1)

			case isa.E16:
				v := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, v-uint16(rs1))

			case isa.E32:
				v := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, v-uint32(rs1))

			case isa.E64:
				v := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, v-uint64(rs1))
			}
		}

	case isa.VrsubVx:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			rs1 := vpu.cpu.X[instr.Rs1]
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				v := vs2reg[vs2ofs]
				reg[ofs] = uint8(rs1) - v

			case isa.E16:
				v := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, uint16(rs1)-v)

			case isa.E32:
				v := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, uint32(rs1)-v)

			case isa.E64:
				v := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, uint64(rs1)-v)
			}
		}

	case isa.VaddVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				vs1 := vs1reg[vs1ofs]
				vs2 := vs2reg[vs2ofs]
				reg[ofs] = vs1 + vs2

			case isa.E16:
				vs1 := memory.Uint16(vs1reg, vs1ofs)
				vs2 := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, vs1+vs2)

			case isa.E32:
				vs1 := memory.Uint32(vs1reg, vs1ofs)
				vs2 := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, vs1+vs2)

			case isa.E64:
				vs1 := memory.Uint64(vs1reg, vs1ofs)
				vs2 := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, vs1+vs2)
			}
		}

	case isa.VsubVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				vs1 := vs1reg[vs1ofs]
				vs2 := vs2reg[vs2ofs]
				reg[ofs] = vs2 - vs1

			case isa.E16:
				vs1 := memory.Uint16(vs1reg, vs1ofs)
				vs2 := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, vs2-vs1)

			case isa.E32:
				vs1 := memory.Uint32(vs1reg, vs1ofs)
				vs2 := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, vs2-vs1)

			case isa.E64:
				vs1 := memory.Uint64(vs1reg, vs1ofs)
				vs2 := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, vs2-vs1)
			}
		}

	case isa.VmulVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				vs1 := vs1reg[vs1ofs]
				vs2 := vs2reg[vs2ofs]
				reg[ofs] = vs2 * vs1

			case isa.E16:
				vs1 := memory.Uint16(vs1reg, vs1ofs)
				vs2 := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, vs2*vs1)

			case isa.E32:
				vs1 := memory.Uint32(vs1reg, vs1ofs)
				vs2 := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, vs2*vs1)

			case isa.E64:
				vs1 := memory.Uint64(vs1reg, vs1ofs)
				vs2 := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, vs2*vs1)
			}
		}

	case isa.VmulhVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				vs1 := vs1reg[vs1ofs]
				vs2 := vs2reg[vs2ofs]
				v := uint8((int16(int8(vs2)) * int16(int8(vs1))) >> sew)
				reg[ofs] = v

			case isa.E16:
				vs1 := memory.Uint16(vs1reg, vs1ofs)
				vs2 := memory.Uint16(vs2reg, vs2ofs)
				v := uint16(int32(int16(vs2)) * int32(int16(vs1)) >> sew)
				memory.PutUint16(reg, ofs, v)

			case isa.E32:
				vs1 := memory.Uint32(vs1reg, vs1ofs)
				vs2 := memory.Uint32(vs2reg, vs2ofs)
				v := uint32(int64(int32(vs2)) * int64(int32(vs1)) >> sew)
				memory.PutUint32(reg, ofs, v)

			case isa.E64:
				vs1 := memory.Uint64(vs1reg, vs1ofs)
				vs2 := memory.Uint64(vs2reg, vs2ofs)
				hi, _ := bits.Mul64(vs1, vs2)

				// Adjust high 64 bits for signed 2's complement inputs:

				// If vs1 is negative, subtract vs2 from high 64 bits.
				if int64(vs1) < 0 {
					hi -= vs2
				}
				// If vs2 is negative, subtract vs1 from high 64 bits.
				if int64(vs2) < 0 {
					hi -= vs1
				}
				memory.PutUint64(reg, ofs, hi)
			}
		}

	case isa.VandnVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				vs1 := vs1reg[vs1ofs]
				vs2 := vs2reg[vs2ofs]
				reg[ofs] = vs2 & ^vs1

			case isa.E16:
				vs1 := memory.Uint16(vs1reg, vs1ofs)
				vs2 := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, vs2&^vs1)

			case isa.E32:
				vs1 := memory.Uint32(vs1reg, vs1ofs)
				vs2 := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, vs2&^vs1)

			case isa.E64:
				vs1 := memory.Uint64(vs1reg, vs1ofs)
				vs2 := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, vs2&^vs1)
			}
		}

	case isa.VandVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				vs1 := vs1reg[vs1ofs]
				vs2 := vs2reg[vs2ofs]
				reg[ofs] = vs1 & vs2

			case isa.E16:
				vs1 := memory.Uint16(vs1reg, vs1ofs)
				vs2 := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, vs1&vs2)

			case isa.E32:
				vs1 := memory.Uint32(vs1reg, vs1ofs)
				vs2 := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, vs1&vs2)

			case isa.E64:
				vs1 := memory.Uint64(vs1reg, vs1ofs)
				vs2 := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, vs1&vs2)
			}
		}

	case isa.VxorVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				vs1 := vs1reg[vs1ofs]
				vs2 := vs2reg[vs2ofs]
				reg[ofs] = vs1 ^ vs2

			case isa.E16:
				vs1 := memory.Uint16(vs1reg, vs1ofs)
				vs2 := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, vs1^vs2)

			case isa.E32:
				vs1 := memory.Uint32(vs1reg, vs1ofs)
				vs2 := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, vs1^vs2)

			case isa.E64:
				vs1 := memory.Uint64(vs1reg, vs1ofs)
				vs2 := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, vs1^vs2)
			}
		}

	case isa.VorVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				vs1 := vs1reg[vs1ofs]
				vs2 := vs2reg[vs2ofs]
				reg[ofs] = vs1 | vs2

			case isa.E16:
				vs1 := memory.Uint16(vs1reg, vs1ofs)
				vs2 := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, vs1|vs2)

			case isa.E32:
				vs1 := memory.Uint32(vs1reg, vs1ofs)
				vs2 := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, vs1|vs2)

			case isa.E64:
				vs1 := memory.Uint64(vs1reg, vs1ofs)
				vs2 := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, vs1|vs2)
			}
		}

	case isa.VrgatherVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)

			switch sew {
			case isa.E8:
				var v uint8
				vs1 := uint64(vs1reg[vs1ofs])
				if vs1 >= vpu.VLMax {
					v = 0
				} else {
					vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), vs1)
					v = vs2reg[vs2ofs]
				}
				reg[ofs] = v

			case isa.E16:
				var v uint16
				vs1 := uint64(memory.Uint16(vs1reg, vs1ofs))
				if vs1 >= vpu.VLMax {
					v = 0
				} else {
					vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), vs1)
					v = memory.Uint16(vs2reg, vs2ofs)
				}
				memory.PutUint16(reg, ofs, v)

			case isa.E32:
				var v uint32
				vs1 := uint64(memory.Uint32(vs1reg, vs1ofs))
				if vs1 >= vpu.VLMax {
					v = 0
				} else {
					vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), vs1)
					v = memory.Uint32(vs2reg, vs2ofs)
				}
				memory.PutUint32(reg, ofs, v)

			case isa.E64:
				var v uint64
				vs1 := uint64(memory.Uint64(vs1reg, vs1ofs))
				if vs1 >= vpu.VLMax {
					v = 0
				} else {
					vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), vs1)
					v = memory.Uint64(vs2reg, vs2ofs)
				}
				memory.PutUint64(reg, ofs, v)
			}
		}

	case isa.Vrgatherei16Vv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs1reg, vs1ofs := vpu.elt(isa.E16, uint64(instr.Rs1), i)

			switch sew {
			case isa.E8:
				var v uint8
				vs1 := uint64(memory.Uint16(vs1reg, vs1ofs))
				if vs1 >= vpu.VLMax {
					v = 0
				} else {
					vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), vs1)
					v = vs2reg[vs2ofs]
				}
				reg[ofs] = v

			case isa.E16:
				var v uint16
				vs1 := uint64(memory.Uint16(vs1reg, vs1ofs))
				if vs1 >= vpu.VLMax {
					v = 0
				} else {
					vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), vs1)
					v = memory.Uint16(vs2reg, vs2ofs)
				}
				memory.PutUint16(reg, ofs, v)

			case isa.E32:
				var v uint32
				vs1 := uint64(memory.Uint16(vs1reg, vs1ofs))
				if vs1 >= vpu.VLMax {
					v = 0
				} else {
					vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), vs1)
					v = memory.Uint32(vs2reg, vs2ofs)
				}
				memory.PutUint32(reg, ofs, v)

			case isa.E64:
				var v uint64
				vs1 := uint64(memory.Uint16(vs1reg, vs1ofs))
				if vs1 >= vpu.VLMax {
					v = 0
				} else {
					vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), vs1)
					v = memory.Uint64(vs2reg, vs2ofs)
				}
				memory.PutUint64(reg, ofs, v)
			}
		}

	case isa.VmaxuVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				vs1 := vs1reg[vs1ofs]
				vs2 := vs2reg[vs2ofs]
				if vs1 > vs2 {
					reg[ofs] = vs1
				} else {
					reg[ofs] = vs2
				}

			case isa.E16:
				vs1 := memory.Uint16(vs1reg, vs1ofs)
				vs2 := memory.Uint16(vs2reg, vs2ofs)
				if vs1 > vs2 {
					memory.PutUint16(reg, ofs, vs1)
				} else {
					memory.PutUint16(reg, ofs, vs2)
				}

			case isa.E32:
				vs1 := memory.Uint32(vs1reg, vs1ofs)
				vs2 := memory.Uint32(vs2reg, vs2ofs)
				if vs1 > vs2 {
					memory.PutUint32(reg, ofs, vs1)
				} else {
					memory.PutUint32(reg, ofs, vs2)
				}

			case isa.E64:
				vs1 := memory.Uint64(vs1reg, vs1ofs)
				vs2 := memory.Uint64(vs2reg, vs2ofs)
				if vs1 > vs2 {
					memory.PutUint64(reg, ofs, vs1)
				} else {
					memory.PutUint64(reg, ofs, vs2)
				}
			}
		}

	case isa.VmaxVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				vs1 := vs1reg[vs1ofs]
				vs2 := vs2reg[vs2ofs]
				if int8(vs1) > int8(vs2) {
					reg[ofs] = vs1
				} else {
					reg[ofs] = vs2
				}

			case isa.E16:
				vs1 := memory.Uint16(vs1reg, vs1ofs)
				vs2 := memory.Uint16(vs2reg, vs2ofs)
				if int16(vs1) > int16(vs2) {
					memory.PutUint16(reg, ofs, vs1)
				} else {
					memory.PutUint16(reg, ofs, vs2)
				}

			case isa.E32:
				vs1 := memory.Uint32(vs1reg, vs1ofs)
				vs2 := memory.Uint32(vs2reg, vs2ofs)
				if int32(vs1) > int32(vs2) {
					memory.PutUint32(reg, ofs, vs1)
				} else {
					memory.PutUint32(reg, ofs, vs2)
				}

			case isa.E64:
				vs1 := memory.Uint64(vs1reg, vs1ofs)
				vs2 := memory.Uint64(vs2reg, vs2ofs)
				if int64(vs1) > int64(vs2) {
					memory.PutUint64(reg, ofs, vs1)
				} else {
					memory.PutUint64(reg, ofs, vs2)
				}
			}
		}

	case isa.VmseqVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)
			var res bool

			switch sew {
			case isa.E8:
				vs1 := vs1reg[vs1ofs]
				vs2 := vs2reg[vs2ofs]
				res = vs1 == vs2

			case isa.E16:
				vs1 := memory.Uint16(vs1reg, vs1ofs)
				vs2 := memory.Uint16(vs2reg, vs2ofs)
				res = vs1 == vs2

			case isa.E32:
				vs1 := memory.Uint32(vs1reg, vs1ofs)
				vs2 := memory.Uint32(vs2reg, vs2ofs)
				res = vs1 == vs2

			case isa.E64:
				vs1 := memory.Uint64(vs1reg, vs1ofs)
				vs2 := memory.Uint64(vs2reg, vs2ofs)
				res = vs1 == vs2
			}
			vpu.setMaskElt(uint64(instr.Rd), i, res)
		}

	case isa.VmsneVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)
			var res bool

			switch sew {
			case isa.E8:
				vs1 := vs1reg[vs1ofs]
				vs2 := vs2reg[vs2ofs]
				res = vs1 != vs2

			case isa.E16:
				vs1 := memory.Uint16(vs1reg, vs1ofs)
				vs2 := memory.Uint16(vs2reg, vs2ofs)
				res = vs1 != vs2

			case isa.E32:
				vs1 := memory.Uint32(vs1reg, vs1ofs)
				vs2 := memory.Uint32(vs2reg, vs2ofs)
				res = vs1 != vs2

			case isa.E64:
				vs1 := memory.Uint64(vs1reg, vs1ofs)
				vs2 := memory.Uint64(vs2reg, vs2ofs)
				res = vs1 != vs2
			}
			vpu.setMaskElt(uint64(instr.Rd), i, res)
		}

	case isa.VmsltuVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)
			var res bool

			switch sew {
			case isa.E8:
				vs1 := vs1reg[vs1ofs]
				vs2 := vs2reg[vs2ofs]
				res = vs2 < vs1

			case isa.E16:
				vs1 := memory.Uint16(vs1reg, vs1ofs)
				vs2 := memory.Uint16(vs2reg, vs2ofs)
				res = vs2 < vs1

			case isa.E32:
				vs1 := memory.Uint32(vs1reg, vs1ofs)
				vs2 := memory.Uint32(vs2reg, vs2ofs)
				res = vs2 < vs1

			case isa.E64:
				vs1 := memory.Uint64(vs1reg, vs1ofs)
				vs2 := memory.Uint64(vs2reg, vs2ofs)
				res = vs2 < vs1
			}
			vpu.setMaskElt(uint64(instr.Rd), i, res)
		}

	case isa.VmsltVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)
			var res bool

			switch sew {
			case isa.E8:
				vs1 := vs1reg[vs1ofs]
				vs2 := vs2reg[vs2ofs]
				res = int8(vs2) < int8(vs1)

			case isa.E16:
				vs1 := memory.Uint16(vs1reg, vs1ofs)
				vs2 := memory.Uint16(vs2reg, vs2ofs)
				res = int16(vs2) < int16(vs1)

			case isa.E32:
				vs1 := memory.Uint32(vs1reg, vs1ofs)
				vs2 := memory.Uint32(vs2reg, vs2ofs)
				res = int32(vs2) < int32(vs1)

			case isa.E64:
				vs1 := memory.Uint64(vs1reg, vs1ofs)
				vs2 := memory.Uint64(vs2reg, vs2ofs)
				res = int64(vs2) < int64(vs1)
			}
			vpu.setMaskElt(uint64(instr.Rd), i, res)
		}

	case isa.VmsleuVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)
			var res bool

			switch sew {
			case isa.E8:
				vs1 := vs1reg[vs1ofs]
				vs2 := vs2reg[vs2ofs]
				res = vs2 <= vs1

			case isa.E16:
				vs1 := memory.Uint16(vs1reg, vs1ofs)
				vs2 := memory.Uint16(vs2reg, vs2ofs)
				res = vs2 <= vs1

			case isa.E32:
				vs1 := memory.Uint32(vs1reg, vs1ofs)
				vs2 := memory.Uint32(vs2reg, vs2ofs)
				res = vs2 <= vs1

			case isa.E64:
				vs1 := memory.Uint64(vs1reg, vs1ofs)
				vs2 := memory.Uint64(vs2reg, vs2ofs)
				res = vs2 <= vs1
			}
			vpu.setMaskElt(uint64(instr.Rd), i, res)
		}

	case isa.VssubuVv:
		sew := vpu.VType.VSEW()
		_ = sew

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			var sat bool

			switch sew {
			case isa.E8:
				vs1 := vs1reg[vs1ofs]
				vs2 := vs2reg[vs2ofs]

				var v uint8
				v, sat = satSubu[uint8](vs2, vs1)
				reg[ofs] = v

			case isa.E16:
				vs1 := memory.Uint16(vs1reg, vs1ofs)
				vs2 := memory.Uint16(vs2reg, vs2ofs)

				var v uint16
				v, sat = satSubu[uint16](vs2, vs1)
				memory.PutUint16(reg, ofs, v)

			case isa.E32:
				vs1 := memory.Uint32(vs1reg, vs1ofs)
				vs2 := memory.Uint32(vs2reg, vs2ofs)

				var v uint32
				v, sat = satSubu[uint32](vs2, vs1)
				memory.PutUint32(reg, ofs, v)

			case isa.E64:
				vs1 := memory.Uint64(vs1reg, vs1ofs)
				vs2 := memory.Uint64(vs2reg, vs2ofs)

				var v uint64
				v, sat = satSubu[uint64](vs2, vs1)
				memory.PutUint64(reg, ofs, v)
			}
			vpu.VXSat = sat
		}

	case isa.VsllVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				vs1 := vs1reg[vs1ofs]
				vs2 := vs2reg[vs2ofs]
				reg[ofs] = vs2 << (vs1 & 7)

			case isa.E16:
				vs1 := memory.Uint16(vs1reg, vs1ofs)
				vs2 := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, vs2<<(vs1&15))

			case isa.E32:
				vs1 := memory.Uint32(vs1reg, vs1ofs)
				vs2 := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, vs2<<(vs1&31))

			case isa.E64:
				vs1 := memory.Uint64(vs1reg, vs1ofs)
				vs2 := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, vs2<<(vs1&63))
			}
		}

	case isa.VsrlVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				vs1 := vs1reg[vs1ofs]
				vs2 := vs2reg[vs2ofs]
				reg[ofs] = vs2 >> (vs1 & 7)

			case isa.E16:
				vs1 := memory.Uint16(vs1reg, vs1ofs)
				vs2 := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, vs2>>(vs1&15))

			case isa.E32:
				vs1 := memory.Uint32(vs1reg, vs1ofs)
				vs2 := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, vs2>>(vs1&31))

			case isa.E64:
				vs1 := memory.Uint64(vs1reg, vs1ofs)
				vs2 := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, vs2>>(vs1&63))
			}
		}

	case isa.VredsumVs:
		sew := vpu.VType.VSEW()

		reg, ofs := vpu.elt(sew, uint64(instr.Rd), 0)
		var vd0res uint64

		vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), 0)
		switch sew {
		case isa.E8:
			vd0res = uint64(vs1reg[vs1ofs])
		case isa.E16:
			vd0res = uint64(memory.Uint16(vs1reg, vs1ofs))
		case isa.E32:
			vd0res = uint64(memory.Uint32(vs1reg, vs1ofs))
		case isa.E64:
			vd0res = memory.Uint64(vs1reg, vs1ofs)
		}

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				vd0res += uint64(vs2reg[vs2ofs])
			case isa.E16:
				vd0res += uint64(memory.Uint16(vs2reg, vs2ofs))
			case isa.E32:
				vd0res += uint64(memory.Uint32(vs2reg, vs2ofs))
			case isa.E64:
				vd0res += uint64(memory.Uint64(vs2reg, vs2ofs))
			}
		}
		if vpu.VL > 0 {
			switch sew {
			case isa.E8:
				reg[ofs] = uint8(vd0res)
			case isa.E16:
				memory.PutUint16(reg, ofs, uint16(vd0res))
			case isa.E32:
				memory.PutUint32(reg, ofs, uint32(vd0res))
			case isa.E64:
				memory.PutUint64(reg, ofs, uint64(vd0res))
			}
		}

	case isa.VredandVs:
		sew := vpu.VType.VSEW()

		reg, ofs := vpu.elt(sew, uint64(instr.Rd), 0)
		var vd0res uint64

		vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), 0)
		switch sew {
		case isa.E8:
			vd0res = uint64(vs1reg[vs1ofs])
		case isa.E16:
			vd0res = uint64(memory.Uint16(vs1reg, vs1ofs))
		case isa.E32:
			vd0res = uint64(memory.Uint32(vs1reg, vs1ofs))
		case isa.E64:
			vd0res = memory.Uint64(vs1reg, vs1ofs)
		}

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				vd0res &= uint64(vs2reg[vs2ofs])
			case isa.E16:
				vd0res &= uint64(memory.Uint16(vs2reg, vs2ofs))
			case isa.E32:
				vd0res &= uint64(memory.Uint32(vs2reg, vs2ofs))
			case isa.E64:
				vd0res &= uint64(memory.Uint64(vs2reg, vs2ofs))
			}
		}
		if vpu.VL > 0 {
			switch sew {
			case isa.E8:
				reg[ofs] = uint8(vd0res)
			case isa.E16:
				memory.PutUint16(reg, ofs, uint16(vd0res))
			case isa.E32:
				memory.PutUint32(reg, ofs, uint32(vd0res))
			case isa.E64:
				memory.PutUint64(reg, ofs, uint64(vd0res))
			}
		}

	case isa.VredorVs:
		sew := vpu.VType.VSEW()

		reg, ofs := vpu.elt(sew, uint64(instr.Rd), 0)
		var vd0res uint64

		vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), 0)
		switch sew {
		case isa.E8:
			vd0res = uint64(vs1reg[vs1ofs])
		case isa.E16:
			vd0res = uint64(memory.Uint16(vs1reg, vs1ofs))
		case isa.E32:
			vd0res = uint64(memory.Uint32(vs1reg, vs1ofs))
		case isa.E64:
			vd0res = memory.Uint64(vs1reg, vs1ofs)
		}

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				vd0res |= uint64(vs2reg[vs2ofs])
			case isa.E16:
				vd0res |= uint64(memory.Uint16(vs2reg, vs2ofs))
			case isa.E32:
				vd0res |= uint64(memory.Uint32(vs2reg, vs2ofs))
			case isa.E64:
				vd0res |= uint64(memory.Uint64(vs2reg, vs2ofs))
			}
		}
		if vpu.VL > 0 {
			switch sew {
			case isa.E8:
				reg[ofs] = uint8(vd0res)
			case isa.E16:
				memory.PutUint16(reg, ofs, uint16(vd0res))
			case isa.E32:
				memory.PutUint32(reg, ofs, uint32(vd0res))
			case isa.E64:
				memory.PutUint64(reg, ofs, uint64(vd0res))
			}
		}

	case isa.VredmaxuVs:
		sew := vpu.VType.VSEW()

		reg, ofs := vpu.elt(sew, uint64(instr.Rd), 0)
		var vd0res uint64

		vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), 0)
		switch sew {
		case isa.E8:
			vd0res = uint64(vs1reg[vs1ofs])
		case isa.E16:
			vd0res = uint64(memory.Uint16(vs1reg, vs1ofs))
		case isa.E32:
			vd0res = uint64(memory.Uint32(vs1reg, vs1ofs))
		case isa.E64:
			vd0res = memory.Uint64(vs1reg, vs1ofs)
		}

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				v := uint64(vs2reg[vs2ofs])
				if v > vd0res {
					vd0res = v
				}
			case isa.E16:
				v := uint64(memory.Uint16(vs2reg, vs2ofs))
				if v > vd0res {
					vd0res = v
				}
			case isa.E32:
				v := uint64(memory.Uint32(vs2reg, vs2ofs))
				if v > vd0res {
					vd0res = v
				}
			case isa.E64:
				v := memory.Uint64(vs2reg, vs2ofs)
				if v > vd0res {
					vd0res = v
				}
			}
		}
		if vpu.VL > 0 {
			switch sew {
			case isa.E8:
				reg[ofs] = uint8(vd0res)
			case isa.E16:
				memory.PutUint16(reg, ofs, uint16(vd0res))
			case isa.E32:
				memory.PutUint32(reg, ofs, uint32(vd0res))
			case isa.E64:
				memory.PutUint64(reg, ofs, uint64(vd0res))
			}
		}

	case isa.VredmaxVs:
		sew := vpu.VType.VSEW()

		reg, ofs := vpu.elt(sew, uint64(instr.Rd), 0)
		var vd0res int64

		vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), 0)
		switch sew {
		case isa.E8:
			vd0res = int64(int8(vs1reg[vs1ofs]))
		case isa.E16:
			vd0res = int64(int16(memory.Uint16(vs1reg, vs1ofs)))
		case isa.E32:
			vd0res = int64(int32(memory.Uint32(vs1reg, vs1ofs)))
		case isa.E64:
			vd0res = int64(memory.Uint64(vs1reg, vs1ofs))
		}

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				v := int64(int8(vs2reg[vs2ofs]))
				if v > vd0res {
					vd0res = v
				}
			case isa.E16:
				v := int64(int16(memory.Uint16(vs2reg, vs2ofs)))
				if v > vd0res {
					vd0res = v
				}
			case isa.E32:
				v := int64(int32(memory.Uint32(vs2reg, vs2ofs)))
				if v > vd0res {
					vd0res = v
				}
			case isa.E64:
				v := int64(memory.Uint64(vs2reg, vs2ofs))
				if v > vd0res {
					vd0res = v
				}
			}
		}
		if vpu.VL > 0 {
			switch sew {
			case isa.E8:
				reg[ofs] = uint8(vd0res)
			case isa.E16:
				memory.PutUint16(reg, ofs, uint16(vd0res))
			case isa.E32:
				memory.PutUint32(reg, ofs, uint32(vd0res))
			case isa.E64:
				memory.PutUint64(reg, ofs, uint64(vd0res))
			}
		}

	case isa.VredxorVs:
		sew := vpu.VType.VSEW()

		reg, ofs := vpu.elt(sew, uint64(instr.Rd), 0)
		var vd0res uint64

		vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), 0)
		switch sew {
		case isa.E8:
			vd0res = uint64(vs1reg[vs1ofs])
		case isa.E16:
			vd0res = uint64(memory.Uint16(vs1reg, vs1ofs))
		case isa.E32:
			vd0res = uint64(memory.Uint32(vs1reg, vs1ofs))
		case isa.E64:
			vd0res = memory.Uint64(vs1reg, vs1ofs)
		}

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				vd0res ^= uint64(vs2reg[vs2ofs])
			case isa.E16:
				vd0res ^= uint64(memory.Uint16(vs2reg, vs2ofs))
			case isa.E32:
				vd0res ^= uint64(memory.Uint32(vs2reg, vs2ofs))
			case isa.E64:
				vd0res ^= uint64(memory.Uint64(vs2reg, vs2ofs))
			}
		}
		if vpu.VL > 0 {
			switch sew {
			case isa.E8:
				reg[ofs] = uint8(vd0res)
			case isa.E16:
				memory.PutUint16(reg, ofs, uint16(vd0res))
			case isa.E32:
				memory.PutUint32(reg, ofs, uint32(vd0res))
			case isa.E64:
				memory.PutUint64(reg, ofs, uint64(vd0res))
			}
		}

	case isa.VfaddVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E32:
				v1 := math.Float32frombits(memory.Uint32(vs1reg, vs1ofs))
				v2 := math.Float32frombits(memory.Uint32(vs2reg, vs2ofs))
				v := math.Float32bits(v1 + v2)
				memory.PutUint32(reg, ofs, v)

			case isa.E64:
				v1 := math.Float64frombits(memory.Uint64(vs1reg, vs1ofs))
				v2 := math.Float64frombits(memory.Uint64(vs2reg, vs2ofs))
				v := math.Float64bits(v1 + v2)
				memory.PutUint64(reg, ofs, v)
			}
		}

	case isa.VfmvFS:
		sew := vpu.VType.VSEW()

		var vs2 float64
		vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), 0)

		switch sew {
		case isa.E32:
			vs2 = float64(math.Float32frombits(memory.Uint32(vs2reg, vs2ofs)))
		case isa.E64:
			vs2 = math.Float64frombits(memory.Uint64(vs2reg, vs2ofs))
		}
		vpu.cpu.F[instr.Rd] = vs2

	case isa.VfcvtFXuV:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}

			switch sew {
			case isa.E32:
				reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
				vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)
				v2 := math.Float32bits(float32(memory.Uint32(vs2reg, vs2ofs)))
				memory.PutUint32(reg, ofs, v2)

			case isa.E64:
				reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
				vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

				v2 := math.Float64bits(float64(memory.Uint64(vs2reg, vs2ofs)))
				memory.PutUint64(reg, ofs, v2)
			}
		}

	case isa.VfmulVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E32:
				v1 := math.Float32frombits(memory.Uint32(vs1reg, vs1ofs))
				v2 := math.Float32frombits(memory.Uint32(vs2reg, vs2ofs))
				v := math.Float32bits(v1 * v2)
				memory.PutUint32(reg, ofs, v)

			case isa.E64:
				v1 := math.Float64frombits(memory.Uint64(vs1reg, vs1ofs))
				v2 := math.Float64frombits(memory.Uint64(vs2reg, vs2ofs))
				v := math.Float64bits(v1 * v2)
				memory.PutUint64(reg, ofs, v)
			}
		}

	case isa.VfnmsubVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E32:
				vd := math.Float32frombits(memory.Uint32(reg, ofs))
				v1 := math.Float32frombits(memory.Uint32(vs1reg, vs1ofs))
				v2 := math.Float32frombits(memory.Uint32(vs2reg, vs2ofs))
				v := math.Float32bits(-(vd * v1) + v2)
				memory.PutUint32(reg, ofs, v)

			case isa.E64:
				vd := math.Float64frombits(memory.Uint64(reg, ofs))
				v1 := math.Float64frombits(memory.Uint64(vs1reg, vs1ofs))
				v2 := math.Float64frombits(memory.Uint64(vs2reg, vs2ofs))
				v := math.Float64bits(-(vd * v1) + v2)
				memory.PutUint64(reg, ofs, v)
			}
		}

	case isa.VfmvSF:
		sew := vpu.VType.VSEW()

		if vpu.VL > 0 && vpu.VStart < vpu.VL {
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), 0)
			v := vpu.cpu.F[instr.Rs1]

			switch sew {
			case isa.E32:
				memory.PutUint32(reg, ofs, math.Float32bits(float32(v)))
			case isa.E64:
				memory.PutUint64(reg, ofs, math.Float64bits(v))
			}
		}

	case isa.VfmergeVfm:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			useFirst := vpu.maskElt(0, i)

			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E32:
				var v uint32
				if useFirst {
					f := float32(vpu.cpu.F[instr.Rs1])
					v = math.Float32bits(f)
				} else {
					v = memory.Uint32(vs2reg, vs2ofs)
				}
				memory.PutUint32(reg, ofs, v)

			case isa.E64:
				var v uint64
				if useFirst {
					f := vpu.cpu.F[instr.Rs1]
					v = math.Float64bits(f)
				} else {
					v = memory.Uint64(vs2reg, vs2ofs)
				}
				memory.PutUint64(reg, ofs, v)
			}
		}

	case isa.VfmvVF:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)

			switch sew {
			case isa.E32:
				f := float32(vpu.cpu.F[instr.Rs1])
				v := math.Float32bits(f)
				memory.PutUint32(reg, ofs, v)

			case isa.E64:
				f := vpu.cpu.F[instr.Rs1]
				v := math.Float64bits(f)
				memory.PutUint64(reg, ofs, v)
			}
		}

	case isa.VmvXS:
		sew := vpu.VType.VSEW()
		vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), 0)
		var val uint64

		switch sew {
		case isa.E8:
			val = uint64(int64(int8(vs2reg[vs2ofs])))
		case isa.E16:
			val = uint64(int64(int16(memory.Uint16(vs2reg, vs2ofs))))
		case isa.E32:
			val = uint64(int64(int32(memory.Uint32(vs2reg, vs2ofs))))
		case isa.E64:
			val = memory.Uint64(vs2reg, vs2ofs)
		}
		vpu.cpu.X[instr.Rd] = val

	case isa.VcpopM:
		var popcount uint64

		if vpu.VStart != 0 {
			return vpu.trap(raw, instr, isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("VStart=%v", vpu.VStart))
		}

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vpu.maskElt(uint64(instr.Rs2), i) {
				if vm == 1 || vpu.maskElt(0, i) {
					popcount++
				}
			}
		}
		vpu.cpu.X[instr.Rd] = popcount

	case isa.VfirstM:
		var pos uint64 = 0xffffffffffffffff

		if vpu.VStart != 0 {
			return vpu.trap(raw, instr, isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("VStart=%v", vpu.VStart))
		}

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			if vpu.maskElt(uint64(instr.Rs2), i) {
				pos = i
				break
			}
		}
		vpu.cpu.X[instr.Rd] = pos

	case isa.VslideupVi:
		sew := vpu.VType.VSEW()
		offset := uint64(imm)

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			if i < offset {
				continue
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i-offset)

			switch sew {
			case isa.E8:
				reg[ofs] = vs2reg[vs2ofs]
			case isa.E16:
				memory.PutUint16(reg, ofs, memory.Uint16(vs2reg, vs2ofs))
			case isa.E32:
				memory.PutUint32(reg, ofs, memory.Uint32(vs2reg, vs2ofs))
			case isa.E64:
				memory.PutUint64(reg, ofs, memory.Uint64(vs2reg, vs2ofs))
			}
		}

	case isa.VslidedownVi:
		sew := vpu.VType.VSEW()
		sh := uint64(imm)

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			var offset uint64
			var isValid bool
			if i+sh < vpu.VLMax {
				isValid = true
			}
			if isValid {
				offset = sh
			}

			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i+offset)

			switch sew {
			case isa.E8:
				var v uint8
				if isValid {
					v = vs2reg[vs2ofs]
				}
				reg[ofs] = v

			case isa.E16:
				var v uint16
				if isValid {
					v = memory.Uint16(vs2reg, vs2ofs)
				}
				memory.PutUint16(reg, ofs, v)

			case isa.E32:
				var v uint32
				if isValid {
					v = memory.Uint32(vs2reg, vs2ofs)
				}
				memory.PutUint32(reg, ofs, v)

			case isa.E64:
				var v uint64
				if isValid {
					v = memory.Uint64(vs2reg, vs2ofs)
				}
				memory.PutUint64(reg, ofs, v)
			}
		}

	case isa.ViotaM:
		sew := vpu.VType.VSEW()
		var cnt uint64

		for i := vpu.VStart; i < vpu.VL; i++ {
			doMask := vpu.maskElt(0, i)

			var hasOne bool
			if vm == 1 || doMask {
				if vpu.maskElt(uint64(instr.Rs2), i) {
					hasOne = true
				}
			}

			// Bypass masked-off elements.
			if vm == 0 && !doMask {
				continue
			}

			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)

			switch sew {
			case isa.E8:
				reg[ofs] = uint8(cnt)
			case isa.E16:
				memory.PutUint16(reg, ofs, uint16(cnt))
			case isa.E32:
				memory.PutUint32(reg, ofs, uint32(cnt))
			case isa.E64:
				memory.PutUint64(reg, ofs, uint64(cnt))
			}

			if hasOne {
				cnt++
			}
		}

	case isa.VidV:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)

			switch sew {
			case isa.E8:
				reg[ofs] = uint8(i)

			case isa.E16:
				memory.PutUint16(reg, ofs, uint16(i))

			case isa.E32:
				memory.PutUint32(reg, ofs, uint32(i))

			case isa.E64:
				memory.PutUint64(reg, ofs, uint64(i))
			}
		}

	case isa.VzextVf8:
		sew := vpu.VType.VSEW()
		from := sew / 8 // div=8
		pat := uint32(((sew >> 3) << 4) | from>>3)

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs2reg, vs2ofs := vpu.elt(from, uint64(instr.Rs2), i)

			switch pat {
			case 0x21:
				memory.PutUint16(reg, ofs, uint16(vs2reg[vs2ofs]))
			case 0x41:
				memory.PutUint32(reg, ofs, uint32(vs2reg[vs2ofs]))
			case 0x81:
				memory.PutUint64(reg, ofs, uint64(vs2reg[vs2ofs]))

			case 0x42:
				memory.PutUint32(reg, ofs,
					uint32(memory.Uint16(vs2reg, vs2ofs)))
			case 0x82:
				memory.PutUint64(reg, ofs,
					uint64(memory.Uint16(vs2reg, vs2ofs)))

			case 0x84:
				memory.PutUint64(reg, ofs,
					uint64(memory.Uint32(vs2reg, vs2ofs)))

			default:
				return vpu.trap(raw, instr, isa.CauseIllegalInstr, uint64(raw),
					fmt.Errorf("SEW=%v", sew))
			}
		}

	case isa.VsextVf8:
		sew := vpu.VType.VSEW()
		from := sew / 8 // div=8
		pat := uint32(((sew >> 3) << 4) | from>>3)

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs2reg, vs2ofs := vpu.elt(from, uint64(instr.Rs2), i)

			switch pat {
			case 0x21:
				memory.PutUint16(reg, ofs, uint16(int16(int8(vs2reg[vs2ofs]))))
			case 0x41:
				memory.PutUint32(reg, ofs, uint32(int32(int8(vs2reg[vs2ofs]))))
			case 0x81:
				memory.PutUint64(reg, ofs, uint64(int64(int8(vs2reg[vs2ofs]))))

			case 0x42:
				memory.PutUint32(reg, ofs,
					uint32(int32(int16(memory.Uint16(vs2reg, vs2ofs)))))
			case 0x82:
				memory.PutUint64(reg, ofs,
					uint64(int64(int16(memory.Uint16(vs2reg, vs2ofs)))))

			case 0x84:
				memory.PutUint64(reg, ofs,
					uint64(int64(int32(memory.Uint32(vs2reg, vs2ofs)))))

			default:
				return vpu.trap(raw, instr, isa.CauseIllegalInstr, uint64(raw),
					fmt.Errorf("SEW=%v", sew))
			}
		}

	case isa.VzextVf4:
		sew := vpu.VType.VSEW()
		from := sew / 4 // div=4
		pat := uint32(((sew >> 3) << 4) | from>>3)

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs2reg, vs2ofs := vpu.elt(from, uint64(instr.Rs2), i)

			switch pat {
			case 0x21:
				memory.PutUint16(reg, ofs, uint16(vs2reg[vs2ofs]))
			case 0x41:
				memory.PutUint32(reg, ofs, uint32(vs2reg[vs2ofs]))
			case 0x81:
				memory.PutUint64(reg, ofs, uint64(vs2reg[vs2ofs]))

			case 0x42:
				memory.PutUint32(reg, ofs,
					uint32(memory.Uint16(vs2reg, vs2ofs)))
			case 0x82:
				memory.PutUint64(reg, ofs,
					uint64(memory.Uint16(vs2reg, vs2ofs)))

			case 0x84:
				memory.PutUint64(reg, ofs,
					uint64(memory.Uint32(vs2reg, vs2ofs)))

			default:
				return vpu.trap(raw, instr, isa.CauseIllegalInstr, uint64(raw),
					fmt.Errorf("SEW=%v", sew))
			}
		}

	case isa.VsextVf4:
		sew := vpu.VType.VSEW()
		from := sew / 4 // div=4
		pat := uint32(((sew >> 3) << 4) | from>>3)

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs2reg, vs2ofs := vpu.elt(from, uint64(instr.Rs2), i)

			switch pat {
			case 0x21:
				memory.PutUint16(reg, ofs, uint16(int16(int8(vs2reg[vs2ofs]))))
			case 0x41:
				memory.PutUint32(reg, ofs, uint32(int32(int8(vs2reg[vs2ofs]))))
			case 0x81:
				memory.PutUint64(reg, ofs, uint64(int64(int8(vs2reg[vs2ofs]))))

			case 0x42:
				memory.PutUint32(reg, ofs,
					uint32(int32(int16(memory.Uint16(vs2reg, vs2ofs)))))
			case 0x82:
				memory.PutUint64(reg, ofs,
					uint64(int64(int16(memory.Uint16(vs2reg, vs2ofs)))))

			case 0x84:
				memory.PutUint64(reg, ofs,
					uint64(int64(int32(memory.Uint32(vs2reg, vs2ofs)))))

			default:
				return vpu.trap(raw, instr, isa.CauseIllegalInstr, uint64(raw),
					fmt.Errorf("SEW=%v", sew))
			}
		}

	case isa.VzextVf2:
		sew := vpu.VType.VSEW()
		from := sew / 2 // div=2
		pat := uint32(((sew >> 3) << 4) | from>>3)

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs2reg, vs2ofs := vpu.elt(from, uint64(instr.Rs2), i)

			switch pat {
			case 0x21:
				memory.PutUint16(reg, ofs, uint16(vs2reg[vs2ofs]))
			case 0x41:
				memory.PutUint32(reg, ofs, uint32(vs2reg[vs2ofs]))
			case 0x81:
				memory.PutUint64(reg, ofs, uint64(vs2reg[vs2ofs]))

			case 0x42:
				memory.PutUint32(reg, ofs,
					uint32(memory.Uint16(vs2reg, vs2ofs)))
			case 0x82:
				memory.PutUint64(reg, ofs,
					uint64(memory.Uint16(vs2reg, vs2ofs)))

			case 0x84:
				memory.PutUint64(reg, ofs,
					uint64(memory.Uint32(vs2reg, vs2ofs)))

			default:
				return vpu.trap(raw, instr, isa.CauseIllegalInstr, uint64(raw),
					fmt.Errorf("SEW=%v", sew))
			}
		}

	case isa.VsextVf2:
		sew := vpu.VType.VSEW()
		from := sew / 2 // div=2
		pat := uint32(((sew >> 3) << 4) | from>>3)

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs2reg, vs2ofs := vpu.elt(from, uint64(instr.Rs2), i)

			switch pat {
			case 0x21:
				memory.PutUint16(reg, ofs, uint16(int16(int8(vs2reg[vs2ofs]))))
			case 0x41:
				memory.PutUint32(reg, ofs, uint32(int32(int8(vs2reg[vs2ofs]))))
			case 0x81:
				memory.PutUint64(reg, ofs, uint64(int64(int8(vs2reg[vs2ofs]))))

			case 0x42:
				memory.PutUint32(reg, ofs,
					uint32(int32(int16(memory.Uint16(vs2reg, vs2ofs)))))
			case 0x82:
				memory.PutUint64(reg, ofs,
					uint64(int64(int16(memory.Uint16(vs2reg, vs2ofs)))))

			case 0x84:
				memory.PutUint64(reg, ofs,
					uint64(int64(int32(memory.Uint32(vs2reg, vs2ofs)))))

			default:
				return vpu.trap(raw, instr, isa.CauseIllegalInstr, uint64(raw),
					fmt.Errorf("SEW=%v", sew))
			}
		}

	case isa.VcpopV:
		sew := vpu.VType.VSEW()
		bits := uint64(sew)

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			var count, j uint64

			switch sew {
			case isa.E8:
				vs2 := vs2reg[vs2ofs]
				for j = 0; j < bits; j++ {
					if vs2>>j&0b1 == 1 {
						count++
					}
				}
				reg[ofs] = uint8(count)

			case isa.E16:
				vs2 := memory.Uint16(vs2reg, vs2ofs)
				for j = 0; j < bits; j++ {
					if vs2>>j&0b1 == 1 {
						count++
					}
				}
				memory.PutUint16(reg, ofs, uint16(count))

			case isa.E32:
				vs2 := memory.Uint32(vs2reg, vs2ofs)
				for j = 0; j < bits; j++ {
					if vs2>>j&0b1 == 1 {
						count++
					}
				}
				memory.PutUint32(reg, ofs, uint32(count))

			case isa.E64:
				vs2 := memory.Uint64(vs2reg, vs2ofs)
				for j = 0; j < bits; j++ {
					if vs2>>j&0b1 == 1 {
						count++
					}
				}
				memory.PutUint64(reg, ofs, count)
			}
		}

	case isa.VcompressVm:
		sew := vpu.VType.VSEW()
		var pos uint64

		if vpu.VStart != 0 {
			return vpu.trap(raw, instr, isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("VStart=%v", vpu.VStart))
		}

		for i := vpu.VStart; i < vpu.VL; i++ {
			if !vpu.maskElt(uint64(instr.Rs1), i) {
				continue
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), pos)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				reg[ofs] = vs2reg[vs2ofs]
			case isa.E16:
				v := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, v)
			case isa.E32:
				v := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, v)
			case isa.E64:
				v := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, v)
			}
			pos++
		}

	case isa.VmandnMm:
		for i := vpu.VStart; i < vpu.VL; i++ {
			vs1 := vpu.maskElt(uint64(instr.Rs1), i)
			vs2 := vpu.maskElt(uint64(instr.Rs2), i)

			vpu.setMaskElt(uint64(instr.Rd), i, vs2 && !vs1)
		}

	case isa.VmandMm:
		for i := vpu.VStart; i < vpu.VL; i++ {
			vs1 := vpu.maskElt(uint64(instr.Rs1), i)
			vs2 := vpu.maskElt(uint64(instr.Rs2), i)

			vpu.setMaskElt(uint64(instr.Rd), i, vs2 && vs1)
		}

	case isa.VmorMm:
		for i := vpu.VStart; i < vpu.VL; i++ {
			vs1 := vpu.maskElt(uint64(instr.Rs1), i)
			vs2 := vpu.maskElt(uint64(instr.Rs2), i)

			vpu.setMaskElt(uint64(instr.Rd), i, vs1 || vs2)
		}

	case isa.VmxorMm:
		for i := vpu.VStart; i < vpu.VL; i++ {
			vs1 := vpu.maskElt(uint64(instr.Rs1), i)
			vs2 := vpu.maskElt(uint64(instr.Rs2), i)

			vpu.setMaskElt(uint64(instr.Rd), i, vs1 != vs2)
		}

	case isa.VmnandMm:
		for i := vpu.VStart; i < vpu.VL; i++ {
			vs1 := vpu.maskElt(uint64(instr.Rs1), i)
			vs2 := vpu.maskElt(uint64(instr.Rs2), i)

			vpu.setMaskElt(uint64(instr.Rd), i, !(vs2 && vs1))
		}

	case isa.VmaddVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				vd := reg[ofs]
				vs1 := vs1reg[vs1ofs]
				vs2 := vs2reg[vs2ofs]
				reg[ofs] = vd*vs1 + vs2

			case isa.E16:
				vd := memory.Uint16(reg, ofs)
				vs1 := memory.Uint16(vs1reg, vs1ofs)
				vs2 := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, vd*vs1+vs2)

			case isa.E32:
				vd := memory.Uint32(reg, ofs)
				vs1 := memory.Uint32(vs1reg, vs1ofs)
				vs2 := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, vd*vs1+vs2)

			case isa.E64:
				vd := memory.Uint64(reg, ofs)
				vs1 := memory.Uint64(vs1reg, vs1ofs)
				vs2 := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, vd*vs1+vs2)
			}
		}

	case isa.VmaccVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				vd := reg[ofs]
				vs1 := vs1reg[vs1ofs]
				vs2 := vs2reg[vs2ofs]
				reg[ofs] = vs1*vs2 + vd

			case isa.E16:
				vd := memory.Uint16(reg, ofs)
				vs1 := memory.Uint16(vs1reg, vs1ofs)
				vs2 := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, vs1*vs2+vd)

			case isa.E32:
				vd := memory.Uint32(reg, ofs)
				vs1 := memory.Uint32(vs1reg, vs1ofs)
				vs2 := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, vs1*vs2+vd)

			case isa.E64:
				vd := memory.Uint64(reg, ofs)
				vs1 := memory.Uint64(vs1reg, vs1ofs)
				vs2 := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, vs1*vs2+vd)
			}
		}

	case isa.VwadduVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				reg, ofs := vpu.elt(isa.E16, uint64(instr.Rd), i)
				vs1 := vs1reg[vs1ofs]
				vs2 := vs2reg[vs2ofs]
				memory.PutUint16(reg, ofs, uint16(vs2)+uint16(vs1))

			case isa.E16:
				reg, ofs := vpu.elt(isa.E32, uint64(instr.Rd), i)
				vs1 := memory.Uint16(vs1reg, vs1ofs)
				vs2 := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, uint32(vs2)+uint32(vs1))

			case isa.E32:
				reg, ofs := vpu.elt(isa.E64, uint64(instr.Rd), i)
				vs1 := memory.Uint32(vs1reg, vs1ofs)
				vs2 := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, uint64(vs2)+uint64(vs1))

			case isa.E64:
				reg, ofs := vpu.elt(isa.E64, uint64(instr.Rd), i)
				vs1 := memory.Uint64(vs1reg, vs1ofs)
				vs2 := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, vs2+vs1)
			}
		}

	case isa.VwsubuVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				reg, ofs := vpu.elt(isa.E16, uint64(instr.Rd), i)
				vs1 := vs1reg[vs1ofs]
				vs2 := vs2reg[vs2ofs]
				memory.PutUint16(reg, ofs, uint16(vs2)-uint16(vs1))

			case isa.E16:
				reg, ofs := vpu.elt(isa.E32, uint64(instr.Rd), i)
				vs1 := memory.Uint16(vs1reg, vs1ofs)
				vs2 := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, uint32(vs2)-uint32(vs1))

			case isa.E32:
				reg, ofs := vpu.elt(isa.E64, uint64(instr.Rd), i)
				vs1 := memory.Uint32(vs1reg, vs1ofs)
				vs2 := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, uint64(vs2)-uint64(vs1))

			case isa.E64:
				reg, ofs := vpu.elt(isa.E64, uint64(instr.Rd), i)
				vs1 := memory.Uint64(vs1reg, vs1ofs)
				vs2 := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, vs2-vs1)
			}
		}

	case isa.VwadduWv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)

			switch sew {
			case isa.E8:
				reg, ofs := vpu.elt(isa.E16, uint64(instr.Rd), i)
				vs2reg, vs2ofs := vpu.elt(isa.E16, uint64(instr.Rs2), i)

				vs1 := uint16(vs1reg[vs1ofs])
				vs2 := memory.Uint16(vs2reg, vs2ofs)

				memory.PutUint16(reg, ofs, vs2+vs1)

			case isa.E16:
				reg, ofs := vpu.elt(isa.E32, uint64(instr.Rd), i)
				vs2reg, vs2ofs := vpu.elt(isa.E32, uint64(instr.Rs2), i)

				vs1 := uint32(memory.Uint16(vs1reg, vs1ofs))
				vs2 := memory.Uint32(vs2reg, vs2ofs)

				memory.PutUint32(reg, ofs, vs2+vs1)

			case isa.E32:
				reg, ofs := vpu.elt(isa.E64, uint64(instr.Rd), i)
				vs2reg, vs2ofs := vpu.elt(isa.E64, uint64(instr.Rs2), i)

				vs1 := uint64(memory.Uint32(vs1reg, vs1ofs))
				vs2 := memory.Uint64(vs2reg, vs2ofs)

				memory.PutUint64(reg, ofs, vs2+vs1)

			case isa.E64:
				reg, ofs := vpu.elt(isa.E64, uint64(instr.Rd), i)
				vs2reg, vs2ofs := vpu.elt(isa.E64, uint64(instr.Rs2), i)

				vs1 := memory.Uint64(vs1reg, vs1ofs)
				vs2 := memory.Uint64(vs2reg, vs2ofs)

				memory.PutUint64(reg, ofs, vs2+vs1)
			}
		}

	case isa.VwaddWv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)

			switch sew {
			case isa.E8:
				reg, ofs := vpu.elt(isa.E16, uint64(instr.Rd), i)
				vs2reg, vs2ofs := vpu.elt(isa.E16, uint64(instr.Rs2), i)

				vs1 := int16(int8(vs1reg[vs1ofs]))
				vs2 := int16(memory.Uint16(vs2reg, vs2ofs))

				memory.PutUint16(reg, ofs, uint16(vs2+vs1))

			case isa.E16:
				reg, ofs := vpu.elt(isa.E32, uint64(instr.Rd), i)
				vs2reg, vs2ofs := vpu.elt(isa.E32, uint64(instr.Rs2), i)

				vs1 := int32(int16(memory.Uint16(vs1reg, vs1ofs)))
				vs2 := int32(memory.Uint32(vs2reg, vs2ofs))

				memory.PutUint32(reg, ofs, uint32(vs2+vs1))

			case isa.E32:
				reg, ofs := vpu.elt(isa.E64, uint64(instr.Rd), i)
				vs2reg, vs2ofs := vpu.elt(isa.E64, uint64(instr.Rs2), i)

				vs1 := int64(int32(memory.Uint32(vs1reg, vs1ofs)))
				vs2 := int64(memory.Uint64(vs2reg, vs2ofs))

				memory.PutUint64(reg, ofs, uint64(vs2+vs1))

			case isa.E64:
				reg, ofs := vpu.elt(isa.E64, uint64(instr.Rd), i)
				vs2reg, vs2ofs := vpu.elt(isa.E64, uint64(instr.Rs2), i)

				vs1 := int64(memory.Uint64(vs1reg, vs1ofs))
				vs2 := int64(memory.Uint64(vs2reg, vs2ofs))

				memory.PutUint64(reg, ofs, uint64(vs2+vs1))
			}
		}

	case isa.VwsubuWv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)

			switch sew {
			case isa.E8:
				reg, ofs := vpu.elt(isa.E16, uint64(instr.Rd), i)
				vs2reg, vs2ofs := vpu.elt(isa.E16, uint64(instr.Rs2), i)

				vs1 := uint16(vs1reg[vs1ofs])
				vs2 := memory.Uint16(vs2reg, vs2ofs)

				memory.PutUint16(reg, ofs, vs2-vs1)

			case isa.E16:
				reg, ofs := vpu.elt(isa.E32, uint64(instr.Rd), i)
				vs2reg, vs2ofs := vpu.elt(isa.E32, uint64(instr.Rs2), i)

				vs1 := uint32(memory.Uint16(vs1reg, vs1ofs))
				vs2 := memory.Uint32(vs2reg, vs2ofs)

				memory.PutUint32(reg, ofs, vs2-vs1)

			case isa.E32:
				reg, ofs := vpu.elt(isa.E64, uint64(instr.Rd), i)
				vs2reg, vs2ofs := vpu.elt(isa.E64, uint64(instr.Rs2), i)

				vs1 := uint64(memory.Uint32(vs1reg, vs1ofs))
				vs2 := memory.Uint64(vs2reg, vs2ofs)

				memory.PutUint64(reg, ofs, vs2-vs1)

			case isa.E64:
				reg, ofs := vpu.elt(isa.E64, uint64(instr.Rd), i)
				vs2reg, vs2ofs := vpu.elt(isa.E64, uint64(instr.Rs2), i)

				vs1 := memory.Uint64(vs1reg, vs1ofs)
				vs2 := memory.Uint64(vs2reg, vs2ofs)

				memory.PutUint64(reg, ofs, vs2-vs1)
			}
		}

	case isa.VwsubWv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)

			switch sew {
			case isa.E8:
				reg, ofs := vpu.elt(isa.E16, uint64(instr.Rd), i)
				vs2reg, vs2ofs := vpu.elt(isa.E16, uint64(instr.Rs2), i)

				vs1 := int16(int8(vs1reg[vs1ofs]))
				vs2 := int16(memory.Uint16(vs2reg, vs2ofs))

				memory.PutUint16(reg, ofs, uint16(vs2-vs1))

			case isa.E16:
				reg, ofs := vpu.elt(isa.E32, uint64(instr.Rd), i)
				vs2reg, vs2ofs := vpu.elt(isa.E32, uint64(instr.Rs2), i)

				vs1 := int32(int16(memory.Uint16(vs1reg, vs1ofs)))
				vs2 := int32(memory.Uint32(vs2reg, vs2ofs))

				memory.PutUint32(reg, ofs, uint32(vs2-vs1))

			case isa.E32:
				reg, ofs := vpu.elt(isa.E64, uint64(instr.Rd), i)
				vs2reg, vs2ofs := vpu.elt(isa.E64, uint64(instr.Rs2), i)

				vs1 := int64(int32(memory.Uint32(vs1reg, vs1ofs)))
				vs2 := int64(memory.Uint64(vs2reg, vs2ofs))

				memory.PutUint64(reg, ofs, uint64(vs2-vs1))

			case isa.E64:
				reg, ofs := vpu.elt(isa.E64, uint64(instr.Rd), i)
				vs2reg, vs2ofs := vpu.elt(isa.E64, uint64(instr.Rs2), i)

				vs1 := int64(memory.Uint64(vs1reg, vs1ofs))
				vs2 := int64(memory.Uint64(vs2reg, vs2ofs))

				memory.PutUint64(reg, ofs, uint64(vs2-vs1))
			}
		}

	case isa.VwmulVv:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs1reg, vs1ofs := vpu.elt(sew, uint64(instr.Rs1), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				reg, ofs := vpu.elt(isa.E16, uint64(instr.Rd), i)
				vs1 := int8(vs1reg[vs1ofs])
				vs2 := int8(vs2reg[vs2ofs])
				memory.PutUint16(reg, ofs, uint16(int16(vs2)*int16(vs1)))

			case isa.E16:
				reg, ofs := vpu.elt(isa.E32, uint64(instr.Rd), i)
				vs1 := int16(memory.Uint16(vs1reg, vs1ofs))
				vs2 := int16(memory.Uint16(vs2reg, vs2ofs))
				memory.PutUint32(reg, ofs, uint32(int32(vs2)*int32(vs1)))

			case isa.E32:
				reg, ofs := vpu.elt(isa.E64, uint64(instr.Rd), i)
				vs1 := int32(memory.Uint32(vs1reg, vs1ofs))
				vs2 := int32(memory.Uint32(vs2reg, vs2ofs))
				memory.PutUint64(reg, ofs, uint64(int64(vs2)*int64(vs1)))

			case isa.E64:
				reg, ofs := vpu.elt(isa.E64, uint64(instr.Rd), i)
				vs1 := int64(memory.Uint64(vs1reg, vs1ofs))
				vs2 := int64(memory.Uint64(vs2reg, vs2ofs))
				memory.PutUint64(reg, ofs, uint64(vs2*vs1))
			}
		}

	case isa.VmvVX:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			rs1 := vpu.cpu.X[instr.Rs1]

			switch sew {
			case isa.E8:
				reg[ofs] = uint8(rs1)

			case isa.E16:
				memory.PutUint16(reg, ofs, uint16(rs1))

			case isa.E32:
				memory.PutUint32(reg, ofs, uint32(rs1))

			case isa.E64:
				memory.PutUint64(reg, ofs, uint64(rs1))
			}
		}

	case isa.VmseqVx:
		sew := vpu.VType.VSEW()
		rs1 := vpu.cpu.X[instr.Rs1]

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)
			var res bool

			switch sew {
			case isa.E8:
				vs2 := vs2reg[vs2ofs]
				res = vs2 == uint8(rs1)

			case isa.E16:
				vs2 := memory.Uint16(vs2reg, vs2ofs)
				res = vs2 == uint16(rs1)

			case isa.E32:
				vs2 := memory.Uint32(vs2reg, vs2ofs)
				res = vs2 == uint32(rs1)

			case isa.E64:
				vs2 := memory.Uint64(vs2reg, vs2ofs)
				res = vs2 == rs1
			}
			vpu.setMaskElt(uint64(instr.Rd), i, res)
		}

	case isa.VmsltuVx:
		sew := vpu.VType.VSEW()
		rs1 := vpu.cpu.X[instr.Rs1]

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)
			var res bool

			switch sew {
			case isa.E8:
				vs2 := vs2reg[vs2ofs]
				res = vs2 < uint8(rs1)

			case isa.E16:
				vs2 := memory.Uint16(vs2reg, vs2ofs)
				res = vs2 < uint16(rs1)

			case isa.E32:
				vs2 := memory.Uint32(vs2reg, vs2ofs)
				res = vs2 < uint32(rs1)

			case isa.E64:
				vs2 := memory.Uint64(vs2reg, vs2ofs)
				res = vs2 < rs1
			}
			vpu.setMaskElt(uint64(instr.Rd), i, res)
		}

	case isa.VnsrlWx:
		sew := vpu.VType.VSEW()
		rs1 := vpu.cpu.X[instr.Rs1] & (uint64(sew)*2 - 1)

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)

			switch sew {
			case isa.E8:
				vs2reg, vs2ofs := vpu.elt(isa.E16, uint64(instr.Rs2), i)
				v := memory.Uint16(vs2reg, vs2ofs)
				reg[ofs] = uint8(v >> rs1)

			case isa.E16:
				vs2reg, vs2ofs := vpu.elt(isa.E32, uint64(instr.Rs2), i)
				v := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, uint16(v>>rs1))

			case isa.E32:
				vs2reg, vs2ofs := vpu.elt(isa.E64, uint64(instr.Rs2), i)
				v := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, uint32(v>>rs1))
			}
		}

	case isa.VmvSX:
		sew := vpu.VType.VSEW()

		if vpu.VL > 0 && vpu.VStart < vpu.VL {
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), 0)
			rs1 := vpu.cpu.X[instr.Rs1]

			switch sew {
			case isa.E8:
				reg[ofs] = uint8(rs1)

			case isa.E16:
				memory.PutUint16(reg, ofs, uint16(rs1))

			case isa.E32:
				memory.PutUint32(reg, ofs, uint32(rs1))

			case isa.E64:
				memory.PutUint64(reg, ofs, uint64(rs1))
			}
		}

	case isa.VmvVI:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)

			switch sew {
			case isa.E8:
				reg[ofs] = uint8(imm)

			case isa.E16:
				memory.PutUint16(reg, ofs, uint16(imm))

			case isa.E32:
				memory.PutUint32(reg, ofs, uint32(imm))

			case isa.E64:
				memory.PutUint64(reg, ofs, uint64(imm))
			}
		}

	case isa.Vle8V:
		lumop := imm
		var eltSize uint64 = 1 // sizeof(uint8)
		veew := eltSize * 8
		vemul := float32(veew) / float32(vpu.VType.VSEW()) * vpu.VType.VLMUL()

		var emul uint64 = 1
		if vemul > 1 {
			emul = uint64(vemul)
		}

		baseAddr := vpu.cpu.X[instr.Rs1]
		vd := uint64(instr.Rd)

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vpu.VStart = i

			for fn := uint64(0); fn < nf; fn++ {
				memAddr := baseAddr + (i*nf+fn)*eltSize
				val, err := vpu.cpu.MMU.Load8(memAddr)
				if err != nil {
					if lumop == 0 || i == 0 {
						// lumop != 0: fault-only-first load
						return err
					}
					// Reduce VL if an extension occurs on a later
					// element. This also terminates the outer loop.
					vpu.VL = i
					break
				}
				reg, ofs := vpu.elt(isa.E8, vd+fn*emul, i)
				reg[ofs] = val
			}
		}

	case isa.Vle16V:
		lumop := imm
		var eltSize uint64 = 2 // sizeof(uint16)
		veew := eltSize * 8
		vemul := float32(veew) / float32(vpu.VType.VSEW()) * vpu.VType.VLMUL()

		var emul uint64 = 1
		if vemul > 1 {
			emul = uint64(vemul)
		}

		baseAddr := vpu.cpu.X[instr.Rs1]
		vd := uint64(instr.Rd)

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vpu.VStart = i

			for fn := uint64(0); fn < nf; fn++ {
				memAddr := baseAddr + (i*nf+fn)*eltSize
				val, err := vpu.cpu.MMU.Load16(memAddr)
				if err != nil {
					if lumop == 0 || i == 0 {
						// lumop != 0: fault-only-first load
						return err
					}
					// Reduce VL if an extension occurs on a later
					// element. This also terminates the outer loop.
					vpu.VL = i
					break
				}
				reg, ofs := vpu.elt(isa.E16, vd+fn*emul, i)
				memory.PutUint16(reg, ofs, val)
			}
		}

	case isa.Vle32V:
		lumop := imm
		var eltSize uint64 = 4 // sizeof(uint32)
		veew := eltSize * 8
		vemul := float32(veew) / float32(vpu.VType.VSEW()) * vpu.VType.VLMUL()

		var emul uint64 = 1
		if vemul > 1 {
			emul = uint64(vemul)
		}

		baseAddr := vpu.cpu.X[instr.Rs1]
		vd := uint64(instr.Rd)

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vpu.VStart = i

			for fn := uint64(0); fn < nf; fn++ {
				memAddr := baseAddr + (i*nf+fn)*eltSize
				val, err := vpu.cpu.MMU.Load32(memAddr)
				if err != nil {
					if lumop == 0 || i == 0 {
						// lumop != 0: fault-only-first load
						return err
					}
					// Reduce VL if an extension occurs on a later
					// element. This also terminates the outer loop.
					vpu.VL = i
					break
				}
				reg, ofs := vpu.elt(isa.E32, vd+fn*emul, i)
				memory.PutUint32(reg, ofs, val)
			}
		}

	case isa.Vle64V:
		lumop := imm
		var eltSize uint64 = 8 // sizeof(uint64)
		veew := eltSize * 8
		vemul := float32(veew) / float32(vpu.VType.VSEW()) * vpu.VType.VLMUL()

		var emul uint64 = 1
		if vemul > 1 {
			emul = uint64(vemul)
		}

		baseAddr := vpu.cpu.X[instr.Rs1]
		vd := uint64(instr.Rd)

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vpu.VStart = i

			for fn := uint64(0); fn < nf; fn++ {
				memAddr := baseAddr + (i*nf+fn)*eltSize
				val, err := vpu.cpu.MMU.Load64(memAddr)
				if err != nil {
					if lumop == 0 || i == 0 {
						// lumop != 0: fault-only-first load
						return err
					}
					// Reduce VL if an extension occurs on a later
					// element. This also terminates the outer loop.
					vpu.VL = i
					break
				}
				reg, ofs := vpu.elt(isa.E64, vd+fn*emul, i)
				memory.PutUint64(reg, ofs, val)
			}
		}

	case isa.Vlse8V:
		var eltSize uint64 = 1 // sizeof(uint8)
		veew := eltSize * 8
		vemul := float32(veew) / float32(vpu.VType.VSEW()) * vpu.VType.VLMUL()

		var emul uint64 = 1
		if vemul > 1 {
			emul = uint64(vemul)
		}

		baseAddr := vpu.cpu.X[instr.Rs1]
		rs2 := vpu.cpu.X[instr.Rs2]
		vd := uint64(instr.Rd)

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vpu.VStart = i

			for fn := uint64(0); fn < nf; fn++ {
				memAddr := baseAddr + i*rs2 + fn*eltSize
				val, err := vpu.cpu.MMU.Load8(memAddr)
				if err != nil {
					return err
				}
				reg, ofs := vpu.elt(isa.E8, vd+fn*emul, i)
				reg[ofs] = val
			}
		}

	case isa.Vlse16V:
		var eltSize uint64 = 2 // sizeof(uint16)
		veew := eltSize * 8
		vemul := float32(veew) / float32(vpu.VType.VSEW()) * vpu.VType.VLMUL()

		var emul uint64 = 1
		if vemul > 1 {
			emul = uint64(vemul)
		}

		baseAddr := vpu.cpu.X[instr.Rs1]
		rs2 := vpu.cpu.X[instr.Rs2]
		vd := uint64(instr.Rd)

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vpu.VStart = i

			for fn := uint64(0); fn < nf; fn++ {
				memAddr := baseAddr + i*rs2 + fn*eltSize
				val, err := vpu.cpu.MMU.Load16(memAddr)
				if err != nil {
					return err
				}
				reg, ofs := vpu.elt(isa.E16, vd+fn*emul, i)
				memory.PutUint16(reg, ofs, val)
			}
		}

	case isa.Vlse32V:
		var eltSize uint64 = 4 // sizeof(uint32)
		veew := eltSize * 8
		vemul := float32(veew) / float32(vpu.VType.VSEW()) * vpu.VType.VLMUL()

		var emul uint64 = 1
		if vemul > 1 {
			emul = uint64(vemul)
		}

		baseAddr := vpu.cpu.X[instr.Rs1]
		rs2 := vpu.cpu.X[instr.Rs2]
		vd := uint64(instr.Rd)

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vpu.VStart = i

			for fn := uint64(0); fn < nf; fn++ {
				memAddr := baseAddr + i*rs2 + fn*eltSize
				val, err := vpu.cpu.MMU.Load32(memAddr)
				if err != nil {
					return err
				}
				reg, ofs := vpu.elt(isa.E32, vd+fn*emul, i)
				memory.PutUint32(reg, ofs, val)
			}
		}

	case isa.Vlse64V:
		var eltSize uint64 = 8 // sizeof(uint64)
		veew := eltSize * 8
		vemul := float32(veew) / float32(vpu.VType.VSEW()) * vpu.VType.VLMUL()

		var emul uint64 = 1
		if vemul > 1 {
			emul = uint64(vemul)
		}

		baseAddr := vpu.cpu.X[instr.Rs1]
		rs2 := vpu.cpu.X[instr.Rs2]
		vd := uint64(instr.Rd)

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vpu.VStart = i

			for fn := uint64(0); fn < nf; fn++ {
				memAddr := baseAddr + i*rs2 + fn*eltSize
				val, err := vpu.cpu.MMU.Load64(memAddr)
				if err != nil {
					return err
				}
				reg, ofs := vpu.elt(isa.E64, vd+fn*emul, i)
				memory.PutUint64(reg, ofs, val)
			}
		}

	case isa.Vl1re8V:
		eltWidth := isa.E8
		baseAddr := vpu.cpu.X[instr.Rs1]
		vd := uint64(instr.Rd)
		eltPerReg := VLEN / 8 / eltWidth.Len()
		size := nf * eltPerReg

		for i := vpu.VStart; i < size; i++ {
			vpu.VStart = i
			val, err := vpu.cpu.MMU.Load8(baseAddr + i*eltWidth.Len())
			if err != nil {
				return err
			}
			reg, ofs := vpu.elt(eltWidth, vd, i)
			reg[ofs] = val
		}

	case isa.Vl1re16V:
		eltWidth := isa.E16
		baseAddr := vpu.cpu.X[instr.Rs1]
		vd := uint64(instr.Rd)
		eltPerReg := VLEN / 8 / eltWidth.Len()
		size := nf * eltPerReg

		for i := vpu.VStart; i < size; i++ {
			vpu.VStart = i
			val, err := vpu.cpu.MMU.Load16(baseAddr + i*eltWidth.Len())
			if err != nil {
				return err
			}
			reg, ofs := vpu.elt(eltWidth, vd, i)
			memory.PutUint16(reg, ofs, val)
		}

	case isa.Vl1re32V:
		eltWidth := isa.E32
		baseAddr := vpu.cpu.X[instr.Rs1]
		vd := uint64(instr.Rd)
		eltPerReg := VLEN / 8 / eltWidth.Len()
		size := nf * eltPerReg

		for i := vpu.VStart; i < size; i++ {
			vpu.VStart = i
			val, err := vpu.cpu.MMU.Load32(baseAddr + i*eltWidth.Len())
			if err != nil {
				return err
			}
			reg, ofs := vpu.elt(eltWidth, vd, i)
			memory.PutUint32(reg, ofs, val)
		}

	case isa.VlmV:
		var eltSize uint64 = 1 // sizeof(uint8)
		veew := eltSize * 8
		vemul := float32(veew) / float32(vpu.VType.VSEW()) * vpu.VType.VLMUL()

		var emul uint64 = 1
		if vemul > 1 {
			emul = uint64(vemul)
		}

		vl := (vpu.VL + 7) / 8
		baseAddr := vpu.cpu.X[instr.Rs1]
		vd := uint64(instr.Rd)

		for i := vpu.VStart; i < vl; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vpu.VStart = i

			for fn := uint64(0); fn < nf; fn++ {
				memAddr := baseAddr + (i*nf+fn)*eltSize
				val, err := vpu.cpu.MMU.Load8(memAddr)
				if err != nil {
					return err
				}
				reg, ofs := vpu.elt(isa.E8, vd+fn*emul, i)
				reg[ofs] = val
			}
		}

	case isa.Vluxei64V:
		var flmul uint64
		vlmul := vpu.VType.VLMUL()
		if vlmul < 1 {
			flmul = 1
		} else {
			flmul = uint64(vlmul)
		}

		eltWidth := isa.E64
		vl := vpu.VL
		baseAddr := vpu.cpu.X[instr.Rs1]
		vd := uint64(instr.Rd)
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vl; i++ {
			ireg, iofs := vpu.elt(eltWidth, uint64(instr.Rs2), i)
			index := memory.Uint64(ireg, iofs)

			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vregInx := i

			vpu.VStart = i
			for fn := uint64(0); fn < nf; fn++ {
				reg, ofs := vpu.elt(sew, vd+fn*flmul, vregInx)
				memAddr := baseAddr + index + fn*sew.Len()

				switch sew {
				case isa.E8:
					val, err := vpu.cpu.MMU.Load8(memAddr)
					if err != nil {
						return err
					}
					reg[ofs] = val

				case isa.E16:
					val, err := vpu.cpu.MMU.Load16(memAddr)
					if err != nil {
						return err
					}
					memory.PutUint16(reg, ofs, val)

				case isa.E32:
					val, err := vpu.cpu.MMU.Load32(memAddr)
					if err != nil {
						return err
					}
					memory.PutUint32(reg, ofs, val)

				case isa.E64:
					val, err := vpu.cpu.MMU.Load64(memAddr)
					if err != nil {
						return err
					}
					memory.PutUint64(reg, ofs, val)
				}
			}
		}

	case isa.Vlnfre64V:
		eltWidth := isa.E64
		baseAddr := vpu.cpu.X[instr.Rs1]
		vd := uint64(instr.Rd)
		eltPerReg := VLEN / 8 / eltWidth.Len()
		size := nf * eltPerReg

		for i := vpu.VStart; i < size; i++ {
			vpu.VStart = i
			val, err := vpu.cpu.MMU.Load64(baseAddr + i*eltWidth.Len())
			if err != nil {
				return err
			}
			reg, ofs := vpu.elt(eltWidth, vd, i)
			memory.PutUint64(reg, ofs, val)
		}

	case isa.Vse8V:
		var eltSize uint64 = 1 // sizeof(uint8)
		veew := eltSize * 8
		vemul := float32(veew) / float32(vpu.VType.VSEW()) * vpu.VType.VLMUL()

		var emul uint64 = 1
		if vemul > 1 {
			emul = uint64(vemul)
		}

		vl := vpu.VL
		baseAddr := vpu.cpu.X[instr.Rs1]
		vs3 := uint64(instr.Rd)

		for i := vpu.VStart; i < vl; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vpu.VStart = i

			for fn := uint64(0); fn < nf; fn++ {
				reg, ofs := vpu.elt(isa.E8, vs3+fn*emul, i)
				val := reg[ofs]

				memAddr := baseAddr + (i*nf+fn)*eltSize
				err := vpu.cpu.MMU.Store8(memAddr, val)
				if err != nil {
					return err
				}
			}
		}

	case isa.Vse16V:
		var eltSize uint64 = 2 // sizeof(uint16)
		veew := eltSize * 8
		vemul := float32(veew) / float32(vpu.VType.VSEW()) * vpu.VType.VLMUL()

		var emul uint64 = 1
		if vemul > 1 {
			emul = uint64(vemul)
		}

		vl := vpu.VL
		baseAddr := vpu.cpu.X[instr.Rs1]
		vs3 := uint64(instr.Rd)

		for i := vpu.VStart; i < vl; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vpu.VStart = i

			for fn := uint64(0); fn < nf; fn++ {
				reg, ofs := vpu.elt(isa.E16, vs3+fn*emul, i)
				val := memory.Uint16(reg, ofs)

				err := vpu.cpu.MMU.Store16(baseAddr+(i*nf+fn)*eltSize, val)
				if err != nil {
					return err
				}
			}
		}

	case isa.Vse32V:
		var eltSize uint64 = 4 // sizeof(uint32)
		veew := eltSize * 8
		vemul := float32(veew) / float32(vpu.VType.VSEW()) * vpu.VType.VLMUL()

		var emul uint64 = 1
		if vemul > 1 {
			emul = uint64(vemul)
		}

		vl := vpu.VL
		baseAddr := vpu.cpu.X[instr.Rs1]
		vs3 := uint64(instr.Rd)

		for i := vpu.VStart; i < vl; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vpu.VStart = i

			for fn := uint64(0); fn < nf; fn++ {
				reg, ofs := vpu.elt(isa.E32, vs3+fn*emul, i)
				val := memory.Uint32(reg, ofs)

				err := vpu.cpu.MMU.Store32(baseAddr+(i*nf+fn)*eltSize, val)
				if err != nil {
					return err
				}
			}
		}

	case isa.Vse64V:
		var eltSize uint64 = 8 // sizeof(uint64)
		veew := eltSize * 8
		vemul := float32(veew) / float32(vpu.VType.VSEW()) * vpu.VType.VLMUL()

		var emul uint64 = 1
		if vemul > 1 {
			emul = uint64(vemul)
		}

		vl := vpu.VL
		baseAddr := vpu.cpu.X[instr.Rs1]
		vs3 := uint64(instr.Rd)

		for i := vpu.VStart; i < vl; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vpu.VStart = i

			for fn := uint64(0); fn < nf; fn++ {
				reg, ofs := vpu.elt(isa.E64, vs3+fn*emul, i)
				val := memory.Uint64(reg, ofs)

				err := vpu.cpu.MMU.Store64(baseAddr+(i*nf+fn)*eltSize, val)
				if err != nil {
					return err
				}
			}
		}

	case isa.Vs1rV:
		baseAddr := vpu.cpu.X[instr.Rs1]
		vs3 := uint64(instr.Rd)
		size := nf * VLEN / 8

		for i := vpu.VStart; i < size; i++ {
			vpu.VStart = i
			reg, ofs := vpu.elt(isa.E8, vs3, i)
			err := vpu.cpu.MMU.Store8(baseAddr+i, reg[ofs])
			if err != nil {
				return err
			}
		}

	case isa.VsmV:
		var eltSize uint64 = 1 // sizeof(uint8)
		veew := eltSize * 8
		vemul := float32(veew) / float32(vpu.VType.VSEW()) * vpu.VType.VLMUL()

		var emul uint64 = 1
		if vemul > 1 {
			emul = uint64(vemul)
		}

		vl := (vpu.VL + 7) / 8
		baseAddr := vpu.cpu.X[instr.Rs1]
		vs3 := uint64(instr.Rd)

		for i := vpu.VStart; i < vl; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vpu.VStart = i

			for fn := uint64(0); fn < nf; fn++ {
				reg, ofs := vpu.elt(isa.E8, vs3+fn*emul, i)
				val := reg[ofs]

				memAddr := baseAddr + (i*nf+fn)*eltSize
				err := vpu.cpu.MMU.Store8(memAddr, val)
				if err != nil {
					return err
				}
			}
		}

	case isa.Vsuxei64V:
		var flmul uint64
		vlmul := vpu.VType.VLMUL()
		if vlmul < 1 {
			flmul = 1
		} else {
			flmul = uint64(vlmul)
		}

		eltWidth := isa.E64
		vl := vpu.VL
		baseAddr := vpu.cpu.X[instr.Rs1]
		vd := uint64(instr.Rd)
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vl; i++ {
			ireg, iofs := vpu.elt(eltWidth, uint64(instr.Rs2), i)
			index := memory.Uint64(ireg, iofs)

			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vregInx := i

			vpu.VStart = i
			for fn := uint64(0); fn < nf; fn++ {
				reg, ofs := vpu.elt(sew, vd+fn*flmul, vregInx)
				memAddr := baseAddr + index + fn*sew.Len()

				switch sew {
				case isa.E8:
					val := reg[ofs]
					err := vpu.cpu.MMU.Store8(memAddr, val)
					if err != nil {
						return err
					}

				case isa.E16:
					val := memory.Uint16(reg, ofs)
					err := vpu.cpu.MMU.Store16(memAddr, val)
					if err != nil {
						return err
					}

				case isa.E32:
					val := memory.Uint32(reg, ofs)
					err := vpu.cpu.MMU.Store32(memAddr, val)
					if err != nil {
						return err
					}

				case isa.E64:
					val := memory.Uint64(reg, ofs)
					err := vpu.cpu.MMU.Store64(memAddr, val)
					if err != nil {
						return err
					}
				}
			}
		}

	case isa.Vsse32V:
		var eltSize uint64 = 4 // sizeof(uint32)
		veew := eltSize * 8
		vemul := float32(veew) / float32(vpu.VType.VSEW()) * vpu.VType.VLMUL()

		var emul uint64 = 1
		if vemul > 1 {
			emul = uint64(vemul)
		}

		vl := vpu.VL
		baseAddr := vpu.cpu.X[instr.Rs1]
		vs3 := uint64(instr.Rd)

		for i := vpu.VStart; i < vl; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vpu.VStart = i
			stride := i * vpu.cpu.X[instr.Rs2]

			for fn := uint64(0); fn < nf; fn++ {
				reg, ofs := vpu.elt(isa.E32, vs3+fn*emul, i)
				val := memory.Uint32(reg, ofs)

				err := vpu.cpu.MMU.Store32(baseAddr+stride+fn*eltSize, val)
				if err != nil {
					return err
				}
			}
		}

	case isa.Vsse64V:
		var eltSize uint64 = 8 // sizeof(uint64)
		veew := eltSize * 8
		vemul := float32(veew) / float32(vpu.VType.VSEW()) * vpu.VType.VLMUL()

		var emul uint64 = 1
		if vemul > 1 {
			emul = uint64(vemul)
		}

		vl := vpu.VL
		baseAddr := vpu.cpu.X[instr.Rs1]
		vs3 := uint64(instr.Rd)

		for i := vpu.VStart; i < vl; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vpu.VStart = i
			stride := i * vpu.cpu.X[instr.Rs2]

			for fn := uint64(0); fn < nf; fn++ {
				reg, ofs := vpu.elt(isa.E64, vs3+fn*emul, i)
				val := memory.Uint64(reg, ofs)

				err := vpu.cpu.MMU.Store64(baseAddr+stride+fn*eltSize, val)
				if err != nil {
					return err
				}
			}
		}

	case isa.VaddVi:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				v := vs2reg[vs2ofs]
				reg[ofs] = uint8(int8(v) + int8(imm))

			case isa.E16:
				v := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, uint16(int16(v)+int16(imm)))

			case isa.E32:
				v := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, uint32(int32(v)+int32(imm)))

			case isa.E64:
				v := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, uint64(int64(v)+int64(imm)))
			}
		}

	case isa.VrsubVi:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				v := vs2reg[vs2ofs]
				reg[ofs] = uint8(int8(imm) - int8(v))

			case isa.E16:
				v := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, uint16(int16(imm)-int16(v)))

			case isa.E32:
				v := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, uint32(int32(imm)-int32(v)))

			case isa.E64:
				v := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, uint64(int64(imm)-int64(v)))
			}
		}

	case isa.VandVi:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				v := vs2reg[vs2ofs]
				reg[ofs] = uint8(int8(imm) & int8(v))

			case isa.E16:
				v := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, uint16(int16(imm)&int16(v)))

			case isa.E32:
				v := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, uint32(int32(imm)&int32(v)))

			case isa.E64:
				v := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, uint64(int64(imm)&int64(v)))
			}
		}

	case isa.VorVi:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				v := vs2reg[vs2ofs]
				reg[ofs] = uint8(int8(imm) | int8(v))

			case isa.E16:
				v := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, uint16(int16(imm)|int16(v)))

			case isa.E32:
				v := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, uint32(int32(imm)|int32(v)))

			case isa.E64:
				v := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, uint64(int64(imm)|int64(v)))
			}
		}

	case isa.VxorVi:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				v := vs2reg[vs2ofs]
				reg[ofs] = uint8(int8(imm) ^ int8(v))

			case isa.E16:
				v := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, uint16(int16(imm)^int16(v)))

			case isa.E32:
				v := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, uint32(int32(imm)^int32(v)))

			case isa.E64:
				v := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, uint64(int64(imm)^int64(v)))
			}
		}

	case isa.VrgatherVi:
		zimm := uint64(imm & 0b11111)
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)

			switch sew {
			case isa.E8:
				var v uint8
				if zimm < vpu.VLMax {
					vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), zimm)
					v = vs2reg[vs2ofs]
				}
				reg[ofs] = v

			case isa.E16:
				var v uint16
				if zimm < vpu.VLMax {
					vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), zimm)
					v = memory.Uint16(vs2reg, vs2ofs)
				}
				memory.PutUint16(reg, ofs, v)

			case isa.E32:
				var v uint32
				if zimm < vpu.VLMax {
					vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), zimm)
					v = memory.Uint32(vs2reg, vs2ofs)
				}
				memory.PutUint32(reg, ofs, v)

			case isa.E64:
				var v uint64
				if zimm < vpu.VLMax {
					vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), zimm)
					v = memory.Uint64(vs2reg, vs2ofs)
				}
				memory.PutUint64(reg, ofs, v)
			}
		}

	case isa.VmseqVi:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)
			var res bool

			switch sew {
			case isa.E8:
				v := vs2reg[vs2ofs]
				res = int8(v) == int8(imm)

			case isa.E16:
				v := memory.Uint16(vs2reg, vs2ofs)
				res = int16(v) == int16(imm)

			case isa.E32:
				v := memory.Uint32(vs2reg, vs2ofs)
				res = int32(v) == int32(imm)

			case isa.E64:
				v := memory.Uint64(vs2reg, vs2ofs)
				res = int64(v) == int64(imm)
			}
			vpu.setMaskElt(uint64(instr.Rd), i, res)
		}

	case isa.VmsneVi:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)
			var res bool

			switch sew {
			case isa.E8:
				v := vs2reg[vs2ofs]
				res = int8(v) != int8(imm)

			case isa.E16:
				v := memory.Uint16(vs2reg, vs2ofs)
				res = int16(v) != int16(imm)

			case isa.E32:
				v := memory.Uint32(vs2reg, vs2ofs)
				res = int32(v) != int32(imm)

			case isa.E64:
				v := memory.Uint64(vs2reg, vs2ofs)
				res = int64(v) != int64(imm)
			}
			vpu.setMaskElt(uint64(instr.Rd), i, res)
		}

	case isa.VmsleuVi:
		sew := vpu.VType.VSEW()
		const uint64Max = 0xffffffffffffffff

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)
			var res bool

			switch sew {
			case isa.E8:
				v := vs2reg[vs2ofs]
				res = v <= uint8(imm&0xff)

			case isa.E16:
				v := memory.Uint16(vs2reg, vs2ofs)
				res = v <= uint16(imm&0xffff)

			case isa.E32:
				v := memory.Uint32(vs2reg, vs2ofs)
				res = v <= uint32(imm)

			case isa.E64:
				v := memory.Uint64(vs2reg, vs2ofs)
				res = v <= uint64(imm)
			}
			vpu.setMaskElt(uint64(instr.Rd), i, res)
		}

	case isa.VmsleVi:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)
			var res bool

			switch sew {
			case isa.E8:
				v := vs2reg[vs2ofs]
				res = int8(v) <= int8(imm)

			case isa.E16:
				v := memory.Uint16(vs2reg, vs2ofs)
				res = int16(v) <= int16(imm)

			case isa.E32:
				v := memory.Uint32(vs2reg, vs2ofs)
				res = int32(v) <= int32(imm)

			case isa.E64:
				v := memory.Uint64(vs2reg, vs2ofs)
				res = int64(v) <= int64(imm)
			}
			vpu.setMaskElt(uint64(instr.Rd), i, res)
		}

	case isa.VmsgtuVi:
		sew := vpu.VType.VSEW()
		const uint64Max = 0xffffffffffffffff

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)
			var res bool

			switch sew {
			case isa.E8:
				v := vs2reg[vs2ofs]
				res = v > uint8(imm&0xff)

			case isa.E16:
				v := memory.Uint16(vs2reg, vs2ofs)
				res = v > uint16(imm&0xffff)

			case isa.E32:
				v := memory.Uint32(vs2reg, vs2ofs)
				res = v > uint32(imm)

			case isa.E64:
				v := memory.Uint64(vs2reg, vs2ofs)
				res = v > uint64(imm)
			}
			vpu.setMaskElt(uint64(instr.Rd), i, res)
		}

	case isa.VmsgtVi:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)
			var res bool

			switch sew {
			case isa.E8:
				v := int8(vs2reg[vs2ofs])
				res = v > int8(imm)

			case isa.E16:
				v := int16(memory.Uint16(vs2reg, vs2ofs))
				res = v > int16(imm)

			case isa.E32:
				v := int32(memory.Uint32(vs2reg, vs2ofs))
				res = v > int32(imm)

			case isa.E64:
				v := int64(memory.Uint64(vs2reg, vs2ofs))
				res = v > int64(imm)
			}
			vpu.setMaskElt(uint64(instr.Rd), i, res)
		}

	case isa.VsllVi:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				v := vs2reg[vs2ofs]
				reg[ofs] = v << (imm & 7)

			case isa.E16:
				v := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, v<<(imm&15))

			case isa.E32:
				v := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, v<<(imm&31))

			case isa.E64:
				v := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, v<<(imm&63))
			}
		}

	case isa.VmvnfrV:
		sew := vpu.VType.VSEW()
		vd := uint64(instr.Rd)
		vs2 := uint64(instr.Rs2)
		numRegs := uint64(instr.Rs1 + 1)
		vlenb := VLEN / 8
		totalBytes := numRegs * vlenb

		// Convert element-based VStart to byte offset. This must be
		// kept in sync with VPU.elt().
		eltBytes := sew.Len()
		startByte := vpu.VStart * eltBytes

		for b := startByte; b < totalBytes; b++ {
			regIdx := vd + (b / vlenb)
			ofs := b % vlenb

			vs2RegIdx := vs2 + (b / vlenb)
			vs2Ofs := b % vlenb

			if regIdx < 32 && vs2RegIdx < 32 {
				vpu.VRegs[regIdx][ofs] = vpu.VRegs[vs2RegIdx][vs2Ofs]
			}
		}

	case isa.VsrlVi:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				v := vs2reg[vs2ofs]
				reg[ofs] = v >> (imm & 7)

			case isa.E16:
				v := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, v>>(imm&15))

			case isa.E32:
				v := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, v>>(imm&31))

			case isa.E64:
				v := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, v>>(imm&63))
			}
		}

	case isa.VsraVi:
		sew := vpu.VType.VSEW()

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				v := vs2reg[vs2ofs]
				reg[ofs] = uint8(int8(v) >> (imm & 7))

			case isa.E16:
				v := memory.Uint16(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, uint16(int16(v)>>(imm&15)))

			case isa.E32:
				v := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, uint32(int32(v)>>(imm&31)))

			case isa.E64:
				v := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint64(reg, ofs, uint64(int64(v)>>(imm&63)))
			}
		}

	case isa.VnsrlWi:
		sew := vpu.VType.VSEW()
		zimm := uint64(imm & 0b11111)
		shift := zimm & (uint64(sew)*2 - 1)

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)

			switch sew {
			case isa.E8:
				vs2reg, vs2ofs := vpu.elt(isa.E16, uint64(instr.Rs2), i)
				v := memory.Uint16(vs2reg, vs2ofs)
				reg[ofs] = uint8(v >> shift)

			case isa.E16:
				vs2reg, vs2ofs := vpu.elt(isa.E32, uint64(instr.Rs2), i)
				v := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, uint16(v>>shift))

			case isa.E32:
				vs2reg, vs2ofs := vpu.elt(isa.E64, uint64(instr.Rs2), i)
				v := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, uint32(v>>shift))
			}
		}

	case isa.VnsraWi:
		sew := vpu.VType.VSEW()
		zimm := uint64(imm & 0b11111)
		shift := zimm & (uint64(sew)*2 - 1)

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)

			switch sew {
			case isa.E8:
				vs2reg, vs2ofs := vpu.elt(isa.E16, uint64(instr.Rs2), i)
				v := memory.Uint16(vs2reg, vs2ofs)
				reg[ofs] = uint8(int16(v) >> shift)

			case isa.E16:
				vs2reg, vs2ofs := vpu.elt(isa.E32, uint64(instr.Rs2), i)
				v := memory.Uint32(vs2reg, vs2ofs)
				memory.PutUint16(reg, ofs, uint16(int32(v)>>shift))

			case isa.E32:
				vs2reg, vs2ofs := vpu.elt(isa.E64, uint64(instr.Rs2), i)
				v := memory.Uint64(vs2reg, vs2ofs)
				memory.PutUint32(reg, ofs, uint32(int64(v)>>shift))
			}
		}

	case isa.VwsllVi:
		sew := vpu.VType.VSEW()
		zimm := uint64(imm & 0b11111)
		shift := zimm & (uint64(sew)*2 - 1)

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vm == 0 {
				if !vpu.maskElt(0, i) {
					continue
				}
			}
			vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), i)

			switch sew {
			case isa.E8:
				v := vs2reg[vs2ofs]
				reg, ofs := vpu.elt(isa.E16, uint64(instr.Rd), i)
				memory.PutUint16(reg, ofs, uint16(v)<<shift)

			case isa.E16:
				v := memory.Uint16(vs2reg, vs2ofs)
				reg, ofs := vpu.elt(isa.E32, uint64(instr.Rd), i)
				memory.PutUint32(reg, ofs, uint32(v)<<shift)

			case isa.E32:
				v := memory.Uint32(vs2reg, vs2ofs)
				reg, ofs := vpu.elt(isa.E64, uint64(instr.Rd), i)
				memory.PutUint64(reg, ofs, uint64(v)<<shift)
			}
		}

	default:
		vpu.cpu.tracef(raw, instr, "not implemented")
		vpu.cpu.Dump(vpu.cpu.PC)
		return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
			fmt.Errorf("instruction %v[0x%x] not implemented yet",
				instr, raw))
	}

	vpu.VStart = 0

	return nil
}

func (vpu *VPU) trap(raw uint32, instr isa.Instr, cause, tval uint64,
	err error) error {
	vpu.cpu.tracef(raw, instr, "%v: %v", isa.ExceptionCauses[cause], err)
	return vpu.cpu.Trap(cause, tval, err)
}

func (vpu *VPU) setVL(rd, rs1 isa.Register, reqVL uint64, newType isa.VType) {

	vlmax := vpu.VLMax

	if vpu.VType != newType {
		vlmul := newType.VLMUL()
		vsew := isa.SEW(math.Min(float64(vlmul)*ELEN, ELEN))
		vlmax = uint64(float32(VLEN/uint64(newType.VSEW())) * vlmul)

		vill := !(vlmul >= 0.125 && vlmul <= 8) ||
			newType.VSEW() > vsew ||
			(newType>>9) != 0 ||
			newType.AltFmt() ||
			(rd == 0 && rs1 == 0 && vpu.VLMax != vlmax)

		if vill {
			vlmax = 0
			vpu.VType = isa.VType(1 << 63)
		} else {
			vpu.VType = newType
		}

		vpu.VLMax = vlmax
	}

	// XXX clear mtype

	if vlmax == 0 {
		vpu.VL = 0
	} else if rd == 0 && rs1 == 0 {
		// Retain current VL.
	} else if rd != 0 && rs1 == 0 {
		vpu.VL = vlmax
	} else if rs1 != 0 {
		if vlmax > reqVL {
			vlmax = reqVL
		}
		vpu.VL = vlmax
	}

	vpu.cpu.X[rd] = vpu.VL
}

func (vpu *VPU) elt(sew isa.SEW, vreg, n uint64) ([]byte, uint64) {
	sewl := uint64(sew) >> 3
	eltsPerReg := (VLEN >> 3) / sewl

	vreg += n / eltsPerReg
	n = n % eltsPerReg

	return vpu.VRegs[vreg], n * sewl
}

func (vpu *VPU) maskElt(vreg, n uint64) bool {
	reg, ofs := vpu.elt(isa.E8, vreg, n/8)

	return (reg[ofs]>>(n%8))&1 == 1
}

func (vpu *VPU) setMaskElt(vreg, n uint64, v bool) {
	reg, ofs := vpu.elt(isa.E8, vreg, n/8)

	bit := byte(1 << (n % 8))

	if v {
		reg[ofs] |= bit
	} else {
		reg[ofs] &^= bit
	}
}

type Unsigned interface {
	uint8 | uint16 | uint32 | uint64
}

func satSubu[T Unsigned](x, y T) (T, bool) {
	res := x - y
	sat := x < y

	if sat {
		res = 0
	}

	return res, sat
}
