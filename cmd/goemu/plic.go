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
	Hart  isa.Hart
	Start uint64
	End   uint64
}

func (plic *PLIC) Halt() error {
	return nil
}

func (plic *PLIC) Contains(paddr uint64) bool {
	return paddr >= plic.Start && paddr < plic.End
}

func (plic *PLIC) Load8(paddr uint64) (uint8, error) {
	if paddr < plic.Start {
		return 0, plic.Hart.Trap(isa.CauseStorePageFault, paddr, nil)
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
		return plic.Hart.Trap(isa.CauseStorePageFault, paddr, nil)
	}
	fmt.Printf("PLIC: 0x%x = 0x%x\r\n", paddr, v)
	return nil
}

func (plic *PLIC) Store16(paddr, v uint64) error {
	fmt.Printf("PLIC: 0x%x = 0x%02x\r\n", paddr, v)
	return nil
}

func (plic *PLIC) Store32(paddr, v uint64) error {
	fmt.Printf("PLIC: 0x%x = 0x%04x\r\n", paddr, v)
	return nil
}

func (plic *PLIC) Store64(paddr, v uint64) error {
	fmt.Printf("PLIC: 0x%x = 0x%08x\r\n", paddr, v)
	return nil
}
