//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package main

import (
	"fmt"

	"github.com/markkurossi/riscv/isa"
)

const (
	PoweroffMagic = 0x555
)

type Syscon struct {
	Hart  isa.Hart
	Start uint64
	End   uint64
}

func (syscon *Syscon) Contains(paddr uint64) bool {
	return paddr >= syscon.Start && paddr < syscon.End
}

func (syscon *Syscon) Load8(paddr uint64) (uint8, error) {
	return 0, nil
}

func (syscon *Syscon) Load16(paddr uint64) (uint16, error) {
	return 0, nil
}

func (syscon *Syscon) Load32(paddr uint64) (uint32, error) {
	return 0, nil
}

func (syscon *Syscon) Load64(paddr uint64) (uint64, error) {
	return 0, nil
}

func (syscon *Syscon) Store8(paddr, v uint64) error {
	if paddr < syscon.Start {
		return syscon.Hart.Trap(isa.CauseStorePageFault, paddr, nil)
	}
	syscon.Hart.Shutdown()
	return nil
}

func (syscon *Syscon) Store16(paddr, v uint64) error {
	syscon.Hart.Shutdown()
	return nil
}

func (syscon *Syscon) Store32(paddr, v uint64) error {
	if v != PoweroffMagic {
		fmt.Printf("Syscon.Store32: 0x%x = 0x%04x\n", paddr, v)
	} else {
		syscon.Hart.Shutdown()
	}
	return nil
}

func (syscon *Syscon) Store64(paddr, v uint64) error {
	syscon.Hart.Shutdown()
	return nil
}
