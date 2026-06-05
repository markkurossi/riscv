//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package cpu

import (
	"github.com/markkurossi/riscv/isa"
)

type VPU struct {
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

func NewVPU() *VPU {
	vpu := &VPU{
		VLEN: 128,
	}
	for i := range vpu.VRegs {
		vpu.VRegs[i] = make([]byte, vpu.VLEN/8)
	}
	return vpu
}
