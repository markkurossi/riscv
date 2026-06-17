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
	_ mmu.ROM = &ROM{}
)

type ROM struct {
	Hart     isa.Hart
	Segments []mmu.ROM
}

func (rom *ROM) Halt() error {
	for _, seg := range rom.Segments {
		err := seg.Halt()
		if err != nil {
			fmt.Printf("halt: %v\n", err)
		}
	}
	return nil
}

func (rom *ROM) Contains(paddr uint64) bool {
	for _, seg := range rom.Segments {
		if seg.Contains(paddr) {
			return true
		}
	}
	return false
}

func (rom *ROM) Load8(paddr uint64) (uint8, error) {
	for _, seg := range rom.Segments {
		if seg.Contains(paddr) {
			return seg.Load8(paddr)
		}
	}
	return 0, rom.Hart.Trap(isa.CauseLoadPageFault, paddr, nil)
}

func (rom *ROM) Load16(paddr uint64) (uint16, error) {
	for _, seg := range rom.Segments {
		if seg.Contains(paddr) {
			return seg.Load16(paddr)
		}
	}
	return 0, rom.Hart.Trap(isa.CauseLoadPageFault, paddr, nil)
}

func (rom *ROM) Load32(paddr uint64) (uint32, error) {
	for _, seg := range rom.Segments {
		if seg.Contains(paddr) {
			return seg.Load32(paddr)
		}
	}
	return 0, rom.Hart.Trap(isa.CauseLoadPageFault, paddr, nil)
}

func (rom *ROM) Load64(paddr uint64) (uint64, error) {
	for _, seg := range rom.Segments {
		if seg.Contains(paddr) {
			return seg.Load64(paddr)
		}
	}
	return 0, rom.Hart.Trap(isa.CauseLoadPageFault, paddr, nil)
}

func (rom *ROM) Store8(paddr uint64, v uint8) error {
	for _, seg := range rom.Segments {
		if seg.Contains(paddr) {
			return seg.Store8(paddr, v)
		}
	}
	return rom.Hart.Trap(isa.CauseStorePageFault, paddr, nil)
}

func (rom *ROM) Store16(paddr uint64, v uint16) error {
	for _, seg := range rom.Segments {
		if seg.Contains(paddr) {
			return seg.Store16(paddr, v)
		}
	}
	return rom.Hart.Trap(isa.CauseStorePageFault, paddr, nil)
}

func (rom *ROM) Store32(paddr uint64, v uint32) error {
	for _, seg := range rom.Segments {
		if seg.Contains(paddr) {
			return seg.Store32(paddr, v)
		}
	}
	return rom.Hart.Trap(isa.CauseStorePageFault, paddr, nil)
}

func (rom *ROM) Store64(paddr uint64, v uint64) error {
	for _, seg := range rom.Segments {
		if seg.Contains(paddr) {
			return seg.Store64(paddr, v)
		}
	}
	return rom.Hart.Trap(isa.CauseStorePageFault, paddr, nil)
}
