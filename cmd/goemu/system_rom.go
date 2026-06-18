//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package main

import (
	"fmt"

	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/mmu"
)

var (
	_ mmu.MMIO = &MMIO{}
)

type MMIO struct {
	Hart     isa.Hart
	Segments []mmu.MMIO
}

func (mmio *MMIO) Halt() error {
	for _, seg := range mmio.Segments {
		err := seg.Halt()
		if err != nil {
			fmt.Printf("halt: %v\n", err)
		}
	}
	return nil
}

func (mmio *MMIO) Contains(paddr uint64) bool {
	for _, seg := range mmio.Segments {
		if seg.Contains(paddr) {
			return true
		}
	}
	return false
}

func (mmio *MMIO) Load8(paddr uint64) (uint8, error) {
	for _, seg := range mmio.Segments {
		if seg.Contains(paddr) {
			return seg.Load8(paddr)
		}
	}
	return 0, mmio.Hart.Trap(isa.CauseLoadPageFault, paddr, nil)
}

func (mmio *MMIO) Load16(paddr uint64) (uint16, error) {
	for _, seg := range mmio.Segments {
		if seg.Contains(paddr) {
			return seg.Load16(paddr)
		}
	}
	return 0, mmio.Hart.Trap(isa.CauseLoadPageFault, paddr, nil)
}

func (mmio *MMIO) Load32(paddr uint64) (uint32, error) {
	for _, seg := range mmio.Segments {
		if seg.Contains(paddr) {
			return seg.Load32(paddr)
		}
	}
	return 0, mmio.Hart.Trap(isa.CauseLoadPageFault, paddr, nil)
}

func (mmio *MMIO) Load64(paddr uint64) (uint64, error) {
	for _, seg := range mmio.Segments {
		if seg.Contains(paddr) {
			return seg.Load64(paddr)
		}
	}
	return 0, mmio.Hart.Trap(isa.CauseLoadPageFault, paddr, nil)
}

func (mmio *MMIO) Store8(paddr uint64, v uint8) error {
	for _, seg := range mmio.Segments {
		if seg.Contains(paddr) {
			return seg.Store8(paddr, v)
		}
	}
	return mmio.Hart.Trap(isa.CauseStorePageFault, paddr, nil)
}

func (mmio *MMIO) Store16(paddr uint64, v uint16) error {
	for _, seg := range mmio.Segments {
		if seg.Contains(paddr) {
			return seg.Store16(paddr, v)
		}
	}
	return mmio.Hart.Trap(isa.CauseStorePageFault, paddr, nil)
}

func (mmio *MMIO) Store32(paddr uint64, v uint32) error {
	for _, seg := range mmio.Segments {
		if seg.Contains(paddr) {
			return seg.Store32(paddr, v)
		}
	}
	return mmio.Hart.Trap(isa.CauseStorePageFault, paddr, nil)
}

func (mmio *MMIO) Store64(paddr uint64, v uint64) error {
	for _, seg := range mmio.Segments {
		if seg.Contains(paddr) {
			return seg.Store64(paddr, v)
		}
	}
	return mmio.Hart.Trap(isa.CauseStorePageFault, paddr, nil)
}
