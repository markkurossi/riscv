//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package cpu

import (
	"fmt"
	"math"

	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/memory"
)

const (
	// ELEN defines the physical width of vector registers in bits.
	ELEN     = 64
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

	// Emulator-specific compile-time configuration
	VLEN uint64 // Physical width of each register in bits (e.g., 128, 256, 512)
}

// NewVPU creates a new Vector Processing Unit for the CPU.
func NewVPU(cpu *CPU) *VPU {
	vpu := &VPU{
		cpu:  cpu,
		VLEN: 128,
	}
	for i := range vpu.VRegs {
		vpu.VRegs[i] = make([]byte, vpu.VLEN/8)
	}
	return vpu
}

func (vpu *VPU) execute(instr isa.Instr, raw uint32) error {
	if vpuDebug {
		vpu.cpu.tracef(raw, instr, "pc=%x", vpu.cpu.PC)
		vpu.cpu.DebugTrace = true
	}
	if vpu.cpu.mstatus.VS() == isa.RegOff {
		return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
			fmt.Errorf("VS=%v", vpu.cpu.mstatus.VS()))
	}

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
		vpu.VStart = 0

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
		vpu.VStart = 0

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
		vpu.VStart = 0

	case isa.VandVx:
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
		vpu.VStart = 0

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
		vpu.VStart = 0

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
		vpu.VStart = 0

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
		vpu.VStart = 0

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
		vpu.VStart = 0

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
		vpu.VStart = 0

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
		vpu.VStart = 0

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
		vpu.VStart = 0

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
		vpu.VStart = 0

	case isa.VrgatherVv:
		vlmul := vpu.VType.VLMUL()
		sew := vpu.VType.VSEW()
		vlmax := uint64(float32(vpu.VLEN/uint64(sew)) * vlmul)

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
				if vs1 >= vlmax {
					v = 0
				} else {
					vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), vs1)
					v = vs2reg[vs2ofs]
				}
				reg[ofs] = v

			case isa.E16:
				var v uint16
				vs1 := uint64(vs1reg[vs1ofs])
				if vs1 >= vlmax {
					v = 0
				} else {
					vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), vs1)
					v = memory.Uint16(vs2reg, vs2ofs)
				}
				memory.PutUint16(reg, ofs, v)

			case isa.E32:
				var v uint32
				vs1 := uint64(vs1reg[vs1ofs])
				if vs1 >= vlmax {
					v = 0
				} else {
					vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), vs1)
					v = memory.Uint32(vs2reg, vs2ofs)
				}
				memory.PutUint32(reg, ofs, v)

			case isa.E64:
				var v uint64
				vs1 := uint64(vs1reg[vs1ofs])
				if vs1 >= vlmax {
					v = 0
				} else {
					vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), vs1)
					v = memory.Uint64(vs2reg, vs2ofs)
				}
				memory.PutUint64(reg, ofs, v)
			}
		}
		vpu.VStart = 0

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
		vpu.VStart = 0

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
		vpu.VStart = 0

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
		vpu.VStart = 0

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
		vpu.VStart = 0

	case isa.VmvXS:
		sew := vpu.VType.VSEW()
		vs2reg, vs2ofs := vpu.elt(sew, uint64(instr.Rs2), 0)
		var val uint64

		switch sew {
		case isa.E8:
			val = uint64(vs2reg[vs2ofs])
		case isa.E16:
			val = uint64(memory.Uint16(vs2reg, vs2ofs))
		case isa.E32:
			val = uint64(memory.Uint32(vs2reg, vs2ofs))
		case isa.E64:
			val = uint64(memory.Uint64(vs2reg, vs2ofs))
		}
		vpu.cpu.X[instr.Rd] = val
		vpu.VStart = 0

	case isa.VcpopM:
		var popcount uint64

		for i := vpu.VStart; i < vpu.VL; i++ {
			if vpu.maskElt(uint64(instr.Rs2), i) {
				if vm == 1 || vpu.maskElt(0, i) {
					popcount++
				}
			}
		}
		vpu.cpu.X[instr.Rd] = popcount
		vpu.VStart = 0

	case isa.VfirstM:
		var pos uint64 = 0xffffffffffffffff

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
		vpu.VStart = 0

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
		vpu.VStart = 0

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
		vpu.VStart = 0

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
				return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
					fmt.Errorf("SEW=%v", sew))
			}
		}
		vpu.VStart = 0

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
				return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
					fmt.Errorf("SEW=%v", sew))
			}
		}
		vpu.VStart = 0

	case isa.VmorMM:
		for i := vpu.VStart; i < vpu.VL; i++ {
			vs1 := vpu.maskElt(uint64(instr.Rs1), i)
			vs2 := vpu.maskElt(uint64(instr.Rs2), i)

			vpu.setMaskElt(uint64(instr.Rd), i, vs1 || vs2)
		}
		vpu.VStart = 0

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
		vpu.VStart = 0

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
		vpu.VStart = 0

	case isa.VmvSX:
		sew := vpu.VType.VSEW()

		if vpu.VStart < vpu.VL {
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
		vpu.VStart = 0

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
		vpu.VStart = 0

	case isa.Vle8V:
		lumop := imm

		if nf != 1 {
			return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("%v: nf=%v", instr, nf))
		}

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
		vpu.VStart = 0

	case isa.Vle16V:
		lumop := imm

		if nf != 1 {
			return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("%v: nf=%v", instr, nf))
		}

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
		vpu.VStart = 0

	case isa.Vle32V:
		lumop := imm

		if nf != 1 {
			return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("%v: nf=%v", instr, nf))
		}

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
		vpu.VStart = 0

	case isa.Vle64V:
		lumop := imm

		if nf != 1 {
			return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("%v: nf=%v", instr, nf))
		}

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
		vpu.VStart = 0

	case isa.Vl1re8V:
		sew := vpu.VType.VSEW()
		baseAddr := vpu.cpu.X[instr.Rs1]
		vd := uint64(instr.Rd)
		eltPerReg := vpu.VLEN / 8 / sew.Len()
		size := nf * eltPerReg

		for i := vpu.VStart; i < size; i++ {
			vpu.VStart = i
			val, err := vpu.cpu.MMU.Load8(baseAddr + i*sew.Len())
			if err != nil {
				return err
			}
			reg, ofs := vpu.elt(sew, vd, i)
			reg[ofs] = val
		}
		vpu.VStart = 0

	case isa.VlmV:
		if nf != 1 {
			return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("%v: nf=%v", instr, nf))
		}

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
		vpu.VStart = 0

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
		vpu.VStart = 0

	case isa.Vse8V:
		if nf != 1 {
			return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("%v: nf=%v", instr, nf))
		}

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
		vpu.VStart = 0

	case isa.Vse16V:
		if nf != 1 {
			return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("%v: nf=%v", instr, nf))
		}

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
		vpu.VStart = 0

	case isa.Vse32V:
		if nf != 1 {
			return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("%v: nf=%v", instr, nf))
		}

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
		vpu.VStart = 0

	case isa.Vse64V:
		if nf != 1 {
			return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("%v: nf=%v", instr, nf))
		}

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
		vpu.VStart = 0

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
		vpu.VStart = 0

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
		vpu.VStart = 0

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
		vpu.VStart = 0

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
		vpu.VStart = 0

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
		vpu.VStart = 0

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
		vpu.VStart = 0

	default:
		vpu.cpu.tracef(raw, instr, "not implemented")
		vpu.cpu.Dump(vpu.cpu.PC)
		return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
			fmt.Errorf("instruction %v[0x%x] not implemented yet",
				instr, raw))
	}

	vpu.cpu.mstatus.SetVS(isa.RegDirty)
	vpu.cpu.mstatus.SetSD(true)

	return nil
}

func (vpu *VPU) requireAlign(raw uint32, val, pos uint64) error {
	if pos == 0 || val&(pos-1) == 0 {
		return nil
	}
	return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
		fmt.Errorf("value %v not aligned at %v", val, pos))
}

func (vpu *VPU) setVL(rd, rs1 isa.Register, reqVL uint64, newType isa.VType) {

	var vl uint64

	if vpu.VType != newType {
		vlmul := newType.VLMUL()
		vsew := isa.SEW(math.Min(float64(vlmul), 1.0) * ELEN)
		vl = uint64(float32(vpu.VLEN/uint64(newType.VSEW())) * vlmul)

		vill := !(vlmul >= 0.125 && vlmul <= 8) ||
			newType.VSEW() > vsew ||
			(newType>>9) != 0 ||
			newType.AltFmt() ||
			(rd == 0 && rs1 == 0 && vpu.VL != vl)

		if vill {
			vl = 0
			vpu.VType = isa.VType(1 << 63)
		} else {
			vpu.VType = newType
		}
	} else {
		vlmul := newType.VLMUL()
		vl = uint64(float32(vpu.VLEN/uint64(newType.VSEW())) * vlmul)
	}

	// XXX clear mtype

	if vl == 0 {
		vpu.VL = 0
	} else if rd == 0 && rs1 == 0 {
		// Retain current VL.
	} else if rd != 0 && rs1 == 0 {
		vpu.VL = vl
	} else if rs1 != 0 {
		if vl > reqVL {
			vl = reqVL
		}
		vpu.VL = vl
	}

	vpu.cpu.X[rd] = vpu.VL
	vpu.VStart = 0
}

func (vpu *VPU) elt(sew isa.SEW, vreg, n uint64) ([]byte, uint64) {
	if vpu.VType.VSEW() == 0 {
		panic("VSEW == 0")
	}
	if vpu.VLEN/sew.Len() == 0 {
		panic("VLEN / SEW == 0")
	}
	eltsPerReg := (vpu.VLEN >> 3) / sew.Len()

	vreg += n / eltsPerReg
	n = n % eltsPerReg

	return vpu.VRegs[vreg], n * sew.Len()
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
