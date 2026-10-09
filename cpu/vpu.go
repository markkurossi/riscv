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
	ELEN     = 64
	vpuDebug = false
)

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
	// Vector extension.

	if vpuDebug {
		vpu.cpu.tracef(raw, instr, "")
	}

	if vpu.cpu.mstatus.VS() == isa.RegOff {
		return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw), nil)
	}

	// Load and store instructions:
	//
	// 	 0:1 vm - 1 unmasked, 0 masked
	// 	 1:3 mop:
	// 	     - 000 unit-stride
	// 	     - 010 strided
	// 	     - 011 indexed (unordered)
	// 	     - 111 indexed (ordered)
	// 	 4:6 nf - number of fields = nf+1

	switch instr.Op {
	case isa.Vsetvli:
		vpu.setVL(instr.Rd, instr.Rs1, vpu.cpu.X[instr.Rs1],
			isa.VType(instr.Imm))

	case isa.Vsetivli:
		vpu.setVL(instr.Rd, 0xff, uint64(instr.Rs1), isa.VType(instr.Imm))

	case isa.VmvVX:
		vl := vpu.VL
		sew := vpu.VType.VSEW()
		scalarVal := vpu.cpu.X[instr.Rs1]
		dest := vpu.VRegs[instr.Rd]

		switch sew {
		case 8:
			val8 := uint8(scalarVal)
			for i := uint64(0); i < vl; i++ {
				dest[i] = val8
			}

		case 16:
			val16 := uint16(scalarVal)
			for i := uint64(0); i < vl; i++ {
				memory.PutUint16(dest, i*2, val16)
			}

		case 32:
			val32 := uint32(scalarVal)
			for i := uint64(0); i < vl; i++ {
				memory.PutUint32(dest, i*4, val32)
			}

		case 64:
			for i := uint64(0); i < vl; i++ {
				memory.PutUint64(dest, i*8, scalarVal)
			}
		}
		vpu.VStart = 0

	case isa.VmvVI:
		vlmul := vpu.VType.VLMUL()
		if vlmul > 1 {
			requireAlign(uint64(instr.Rd), uint64(vlmul))
			requireAlign(uint64(instr.Rs2), uint64(vlmul))
		}
		sew := vpu.VType.VSEW()
		if sew < isa.E8 || sew > isa.E64 {
			return fmt.Errorf("SEW=%v", sew)
		}

		for i := vpu.VStart; i < vpu.VL; i++ {
			reg, ofs := vpu.elt(sew, uint64(instr.Rd), i)

			switch sew {
			case isa.E8:
				reg[ofs] = uint8(instr.Imm)

			case isa.E16:
				memory.PutUint16(reg, ofs, uint16(instr.Imm))

			case isa.E32:
				memory.PutUint32(reg, ofs, uint32(instr.Imm))

			case isa.E64:
				memory.PutUint64(reg, ofs, uint64(instr.Imm))
			}
		}
		vpu.VStart = 0

	case isa.Vle8V:
		vm := instr.Imm & 0b1
		mop := instr.Imm >> 1 & 0b111
		nf := instr.Imm >> 4 & 0b111

		if vm != 1 || mop != 0 || nf != 0 {
			return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("instruction %v not implemented yet", instr))
		}

		baseAddr := vpu.cpu.X[instr.Rs1]
		vl := vpu.VL
		dstVec := vpu.VRegs[instr.Rd]

		for i := vpu.VStart; i < vl; i++ {
			srcAddr := baseAddr + i
			val, err := vpu.cpu.MMU.Load8(srcAddr)
			if err != nil {
				vpu.VStart = i
				return err
			}
			dstVec[i] = val
		}
		vpu.VStart = 0

	case isa.Vse8V:
		vm := instr.Imm & 0b1
		mop := instr.Imm >> 1 & 0b111
		nf := instr.Imm >> 4 & 0b111

		if vm != 1 || mop != 0 || nf != 0 {
			return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("instruction %v not implemented yet", instr))
		}

		baseAddr := vpu.cpu.X[instr.Rs1]
		vl := vpu.VL
		srcVec := vpu.VRegs[instr.Rd]

		for i := vpu.VStart; i < vl; i++ {
			if i+1 > uint64(len(srcVec)) {
				return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw), nil)
			}
			v := srcVec[i]

			targetAddr := baseAddr + i
			err := vpu.cpu.MMU.Store8(targetAddr, v)
			if err != nil {
				vpu.cpu.tracef(raw, instr, "store: base=%x, i=%v, vl=%v",
					baseAddr, i, vpu.VL)
				vpu.VStart = i
				return err
			}
		}
		vpu.VStart = 0

	case isa.Vse64V:
		vm := instr.Imm & 0b1
		mop := instr.Imm >> 1 & 0b111
		nf := instr.Imm >> 4 & 0b111

		if vm != 1 || mop != 0 || nf != 0 {
			return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("instruction %v not implemented yet", instr))
		}

		baseAddr := vpu.cpu.X[instr.Rs1]
		vl := vpu.VL
		srcVec := vpu.VRegs[instr.Rd]

		for i := vpu.VStart; i < vl; i++ {
			elementOfs := i * 8
			if elementOfs+8 > uint64(len(srcVec)) {
				return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw), nil)
			}
			v := memory.Uint64(srcVec, elementOfs)

			targetAddr := baseAddr + i*8
			err := vpu.cpu.MMU.Store64(targetAddr, v)
			if err != nil {
				vpu.VStart = i
				return err
			}
		}
		vpu.VStart = 0
	}

	vpu.cpu.mstatus.SetVS(isa.RegDirty)
	vpu.cpu.mstatus.SetSD(true)

	return nil
}

func requireAlign(val, pos uint64) error {
	if pos == 0 || val&(pos-1) == 0 {
		return nil
	}
	return fmt.Errorf("value %v not aligned at %v", val, pos)
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
			vpu.VType = -1
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

	return vpu.VRegs[vreg], n
}
