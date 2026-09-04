//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package cpu

import (
	"fmt"

	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/memory"
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

	vpu.cpu.tracef(raw, instr, "")

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
		vtype := isa.VType(instr.Imm)
		vpu.VType = vtype
		maxVL := uint64(float32(vpu.VLEN)*vtype.VLMUL()) /
			uint64(vtype.VSEW())

		requestedVL := vpu.cpu.X[instr.Rs1]
		if requestedVL > maxVL {
			vpu.VL = maxVL
		} else {
			vpu.VL = requestedVL
		}
		vpu.cpu.X[instr.Rd] = vpu.VL
		vpu.VStart = 0

	case isa.Vsetivli:
		vtype := isa.VType(instr.Imm)
		vpu.VType = vtype
		maxVL := uint64(float32(vpu.VLEN)*vtype.VLMUL()) /
			uint64(vtype.VSEW())

		var requestedVL uint64
		if instr.Rs1 == 0 {
			requestedVL = maxVL
		} else {
			requestedVL = vpu.cpu.X[instr.Rs1]
		}

		if requestedVL > maxVL {
			vpu.VL = maxVL
		} else {
			vpu.VL = requestedVL
		}
		vpu.cpu.X[instr.Rd] = vpu.VL
		vpu.VStart = 0

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
		vl := vpu.VL
		sew := vpu.VType.VSEW()
		dest := vpu.VRegs[instr.Rd]

		switch sew {
		case 8:
			val8 := uint8(instr.Imm)
			for i := vpu.VStart; i < vl; i++ {
				dest[i] = val8
			}

		case 16:
			val16 := uint16(instr.Imm)
			for i := vpu.VStart; i < vl; i++ {
				memory.PutUint16(dest, i*2, val16)
			}

		case 32:
			val32 := uint32(instr.Imm)
			for i := vpu.VStart; i < vl; i++ {
				memory.PutUint32(dest, i*4, val32)
			}

		case 64:
			val64 := uint64(instr.Imm)
			for i := vpu.VStart; i < vl; i++ {
				memory.PutUint64(dest, i*8, val64)
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
		vl := vpu.VL / 8
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
		vl := vpu.VL / 8
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
		vl := vpu.VL / 64
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
