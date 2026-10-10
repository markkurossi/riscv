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
		vpu.cpu.tracef(raw, instr, "pc=%x", vpu.cpu.PC)
		vpu.cpu.DebugTrace = true
	}

	if vpu.cpu.mstatus.VS() == isa.RegOff {
		return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
			fmt.Errorf("VS=%v", vpu.cpu.mstatus.VS()))
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
	case isa.Vsetvli: // ok
		vpu.setVL(instr.Rd, instr.Rs1, vpu.cpu.X[instr.Rs1],
			isa.VType(instr.Imm))

	case isa.Vsetivli: // ok
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

	case isa.VmvVI: // ok
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

	case isa.Vle8V: // XXX ok?
		vm := instr.Imm & 0b1
		mop := instr.Imm >> 1 & 0b111
		nf := uint64(instr.Imm>>4&0b111) + 1

		if vm != 1 || mop != 0 || nf != 1 {
			return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("instruction %v not implemented yet", instr))
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
		vd := uint64(instr.Rd)

		for i := vpu.VStart; i < vl; i++ {
			vpu.VStart = i

			for fn := uint64(0); fn < nf; fn++ {
				memAddr := baseAddr + (i*nf+fn)*eltSize
				pc := vpu.cpu.PC
				val, err := vpu.cpu.MMU.Load8(memAddr)
				if err != nil {
					vpu.cpu.pctracef(pc, raw, instr,
						"load: addr=%x, i=%v, vl=%v",
						memAddr, i, vpu.VL)
					return err
				}
				reg, ofs := vpu.elt(isa.E8, vd+fn*emul, i)
				reg[ofs] = val
			}
		}
		vpu.VStart = 0

	case isa.Vle16V: // XXX ok?
		vm := instr.Imm & 0b1
		mop := instr.Imm >> 1 & 0b111
		nf := uint64(instr.Imm>>4&0b111) + 1

		if vm != 1 || mop != 0 || nf != 1 {
			return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("instruction %v not implemented yet", instr))
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
		vd := uint64(instr.Rd)

		for i := vpu.VStart; i < vl; i++ {
			vpu.VStart = i

			for fn := uint64(0); fn < nf; fn++ {
				memAddr := baseAddr + (i*nf+fn)*eltSize
				val, err := vpu.cpu.MMU.Load16(memAddr)
				if err != nil {
					return err
				}
				reg, ofs := vpu.elt(isa.E16, vd+fn*emul, i)
				memory.PutUint16(reg, ofs, val)
			}
		}
		vpu.VStart = 0

	case isa.Vle32V: // XXX ok?
		vm := instr.Imm & 0b1
		mop := instr.Imm >> 1 & 0b111
		nf := uint64(instr.Imm>>4&0b111) + 1

		if vm != 1 || mop != 0 || nf != 1 {
			return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("instruction %v not implemented yet", instr))
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
		vd := uint64(instr.Rd)

		for i := vpu.VStart; i < vl; i++ {
			vpu.VStart = i

			for fn := uint64(0); fn < nf; fn++ {
				memAddr := baseAddr + (i*nf+fn)*eltSize
				val, err := vpu.cpu.MMU.Load32(memAddr)
				if err != nil {
					return err
				}
				reg, ofs := vpu.elt(isa.E32, vd+fn*emul, i)
				memory.PutUint32(reg, ofs, val)
			}
		}
		vpu.VStart = 0

	case isa.Vle64V: // XXX ok?
		vm := instr.Imm & 0b1
		mop := instr.Imm >> 1 & 0b111
		nf := uint64(instr.Imm>>4&0b111) + 1

		if vm != 1 || mop != 0 || nf != 1 {
			return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("instruction %v not implemented yet", instr))
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
		vd := uint64(instr.Rd)

		for i := vpu.VStart; i < vl; i++ {
			vpu.VStart = i

			for fn := uint64(0); fn < nf; fn++ {
				memAddr := baseAddr + (i*nf+fn)*eltSize
				val, err := vpu.cpu.MMU.Load64(memAddr)
				if err != nil {
					return err
				}
				reg, ofs := vpu.elt(isa.E64, vd+fn*emul, i)
				memory.PutUint64(reg, ofs, val)
			}
		}
		vpu.VStart = 0

	case isa.Vse8V: // XXX ok?
		vm := instr.Imm & 0b1
		mop := instr.Imm >> 1 & 0b111
		nf := uint64(instr.Imm>>4&0b111) + 1

		if vm != 1 || mop != 0 || nf != 1 {
			return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("not implemented yet: vm=%v, mop=%v, nf=%v",
					vm, mop, nf))
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
			vpu.VStart = i

			for fn := uint64(0); fn < nf; fn++ {
				reg, ofs := vpu.elt(isa.E8, vs3+fn*emul, i)
				val := reg[ofs]

				memAddr := baseAddr + (i*nf+fn)*eltSize
				pc := vpu.cpu.PC
				err := vpu.cpu.MMU.Store8(memAddr, val)
				if err != nil {
					vpu.cpu.pctracef(pc, raw, instr,
						"store: base=%x, addr=%x, i=%v, vl=%v",
						baseAddr, memAddr, i, vpu.VL)
					return err
				}
			}
		}
		vpu.VStart = 0

	case isa.Vse16V: // XXX ok?
		vm := instr.Imm & 0b1
		mop := instr.Imm >> 1 & 0b111
		nf := uint64(instr.Imm>>4&0b111) + 1

		if vm != 1 || mop != 0 || nf != 1 {
			return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("not implemented yet: vm=%v, mop=%v, nf=%v",
					vm, mop, nf))
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
			vpu.VStart = i

			for fn := uint64(0); fn < nf; fn++ {
				reg, ofs := vpu.elt(isa.E16, vs3+fn*emul, i)
				val := memory.Uint16(reg, ofs)

				err := vpu.cpu.MMU.Store16(baseAddr+(i*nf+fn)*eltSize, val)
				if err != nil {
					vpu.cpu.tracef(raw, instr, "store: base=%x, i=%v, vl=%v",
						baseAddr, i, vpu.VL)
					return err
				}
			}
		}
		vpu.VStart = 0

	case isa.Vse32V: // XXX ok?
		vm := instr.Imm & 0b1
		mop := instr.Imm >> 1 & 0b111
		nf := uint64(instr.Imm>>4&0b111) + 1

		if vm != 1 || mop != 0 || nf != 1 {
			return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("not implemented yet: vm=%v, mop=%v, nf=%v",
					vm, mop, nf))
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
			vpu.VStart = i

			for fn := uint64(0); fn < nf; fn++ {
				reg, ofs := vpu.elt(isa.E32, vs3+fn*emul, i)
				val := memory.Uint32(reg, ofs)

				err := vpu.cpu.MMU.Store32(baseAddr+(i*nf+fn)*eltSize, val)
				if err != nil {
					vpu.cpu.tracef(raw, instr, "store: base=%x, i=%v, vl=%v",
						baseAddr, i, vpu.VL)
					return err
				}
			}
		}
		vpu.VStart = 0

	case isa.Vse64V: // XXX ok?
		vm := instr.Imm & 0b1
		mop := instr.Imm >> 1 & 0b111
		nf := uint64(instr.Imm>>4&0b111) + 1

		if vm != 1 || mop != 0 || nf != 1 {
			return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("not implemented yet: vm=%v, mop=%v, nf=%v",
					vm, mop, nf))
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
			vpu.VStart = i

			for fn := uint64(0); fn < nf; fn++ {
				reg, ofs := vpu.elt(isa.E64, vs3+fn*emul, i)
				val := memory.Uint64(reg, ofs)

				err := vpu.cpu.MMU.Store64(baseAddr+(i*nf+fn)*eltSize, val)
				if err != nil {
					vpu.cpu.tracef(raw, instr, "store: base=%x, i=%v, vl=%v",
						baseAddr, i, vpu.VL)
					return err
				}
			}
		}
		vpu.VStart = 0

	default:
		if false {
			return vpu.cpu.Trap(isa.CauseIllegalInstr, uint64(raw),
				fmt.Errorf("%v not implemented yet", instr))
		}
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

	return vpu.VRegs[vreg], n * sew.Len()
}
