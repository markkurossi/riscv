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

type CLINT struct {
	Start uint64
	End   uint64
}

func (clint *CLINT) Contains(paddr uint64) bool {
	return paddr >= clint.Start && paddr < clint.End
}

func (clint *CLINT) Load8(paddr uint64) (uint8, error) {
	if paddr < clint.Start {
		return 0, isa.NewTrap(0, 0, isa.CauseStorePageFault, paddr, nil)
	}
	fmt.Printf("CLINT.Load8: 0x%x\n", paddr)
	return 0, nil
}

func (clint *CLINT) Load16(paddr uint64) (uint16, error) {
	if paddr < clint.Start {
		return 0, isa.NewTrap(0, 0, isa.CauseStorePageFault, paddr, nil)
	}
	fmt.Printf("CLINT.Load16: 0x%x\n", paddr)
	return 0, nil
}

func (clint *CLINT) Load32(paddr uint64) (uint32, error) {
	if paddr < clint.Start {
		return 0, isa.NewTrap(0, 0, isa.CauseStorePageFault, paddr, nil)
	}
	fmt.Printf("CLINT.Load32: 0x%x\n", paddr)
	return 0, nil
}

func (clint *CLINT) Load64(paddr uint64) (uint64, error) {
	if paddr < clint.Start {
		return 0, isa.NewTrap(0, 0, isa.CauseStorePageFault, paddr, nil)
	}
	fmt.Printf("CLINT.Load64: 0x%x\n", paddr)
	return 0, nil
}

func (clint *CLINT) Store8(paddr, v uint64) error {
	if paddr < clint.Start {
		return isa.NewTrap(0, 0, isa.CauseStorePageFault, paddr, nil)
	}
	fmt.Printf("CLINT.Store8: 0x%x = 0x%x\n", paddr, v)
	return nil
}

func (clint *CLINT) Store16(paddr, v uint64) error {
	fmt.Printf("CLINT.Store16: 0x%x = 0x%02x\n", paddr, v)
	return nil
}

func (clint *CLINT) Store32(paddr, v uint64) error {
	fmt.Printf("CLINT.Store32: 0x%x = 0x%04x\n", paddr, v)
	return nil
}

func (clint *CLINT) Store64(paddr, v uint64) error {
	fmt.Printf("CLINT.Store64: 0x%x = 0x%08x\n", paddr, v)
	return nil
}
