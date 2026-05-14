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

type PLIC struct {
	Start uint64
	End   uint64
}

func (plic *PLIC) Contains(paddr uint64) bool {
	return paddr >= plic.Start && paddr < plic.End
}

func (plic *PLIC) Load8(paddr uint64) (uint8, error) {
	if paddr < plic.Start {
		return 0, isa.NewTrap(0, 0, isa.CauseStorePageFault, paddr, nil)
	}
	return 0, nil
}

func (plic *PLIC) Load16(paddr uint64) (uint16, error) {
	return 0, nil
}

func (plic *PLIC) Load32(paddr uint64) (uint32, error) {
	return 0, nil
}

func (plic *PLIC) Load64(paddr uint64) (uint64, error) {
	return 0, nil
}

func (plic *PLIC) Store8(paddr, v uint64) error {
	if paddr < plic.Start {
		return isa.NewTrap(0, 0, isa.CauseStorePageFault, paddr, nil)
	}
	fmt.Printf("PLIC: 0x%x = 0x%x\n", paddr, v)
	return nil
}

func (plic *PLIC) Store16(paddr, v uint64) error {
	fmt.Printf("PLIC: 0x%x = 0x%02x\n", paddr, v)
	return nil
}

func (plic *PLIC) Store32(paddr, v uint64) error {
	fmt.Printf("PLIC: 0x%x = 0x%04x\n", paddr, v)
	return nil
}

func (plic *PLIC) Store64(paddr, v uint64) error {
	fmt.Printf("PLIC: 0x%x = 0x%08x\n", paddr, v)
	return nil
}
