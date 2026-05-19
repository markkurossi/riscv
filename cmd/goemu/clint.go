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
	ClintOfsMsip     = 0
	ClintOfsMtimecmp = 0x4000
	ClintOfsMtime    = 0xbff8
)

type CLINT struct {
	Hart  isa.Hart
	Start uint64
	End   uint64

	Msip     uint64
	Mtimecmp uint64
	Mtime    uint64
}

func (clint *CLINT) Contains(paddr uint64) bool {
	return paddr >= clint.Start && paddr < clint.End
}

func (clint *CLINT) load(paddr uint64) (uint64, error) {
	if !clint.Contains(paddr) {
		return 0, clint.Hart.Trap(isa.CauseLoadPageFault, paddr, nil)
	}

	var v uint64

	ofs := paddr - clint.Start
	switch ofs {
	case ClintOfsMsip:
		v = clint.Msip

	case ClintOfsMtimecmp:
		v = clint.Mtimecmp

	case ClintOfsMtime:
		v = clint.Mtime

	default:
		fmt.Printf("CLINT: load: unknown register %x\n", ofs)
	}

	fmt.Printf("CLINT.load(0x%x) => 0x%x\n", ofs, v)

	return v, nil
}

func (clint *CLINT) store(paddr, v uint64) error {
	if !clint.Contains(paddr) {
		return clint.Hart.Trap(isa.CauseStorePageFault, paddr, nil)
	}
	ofs := paddr - clint.Start

	fmt.Printf("CLINT.store(0x%x, 0x%x)\n", ofs, v)

	switch ofs {
	case ClintOfsMsip:
		clint.Msip = v

	case ClintOfsMtimecmp:
		clint.Mtimecmp = v

	case ClintOfsMtime:
		clint.Mtime = v

	default:
		fmt.Printf("CLINT: store: unknown register %x = %x\n", ofs, v)
	}

	return nil
}

func (clint *CLINT) Load8(paddr uint64) (uint8, error) {
	v, err := clint.load(paddr)
	if err != nil {
		return 0, err
	}
	return uint8(v), nil
}

func (clint *CLINT) Load16(paddr uint64) (uint16, error) {
	v, err := clint.load(paddr)
	if err != nil {
		return 0, err
	}
	return uint16(v), nil
}

func (clint *CLINT) Load32(paddr uint64) (uint32, error) {
	v, err := clint.load(paddr)
	if err != nil {
		return 0, err
	}
	return uint32(v), nil
}

func (clint *CLINT) Load64(paddr uint64) (uint64, error) {
	v, err := clint.load(paddr)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func (clint *CLINT) Store8(paddr, v uint64) error {
	return clint.store(paddr, v)
}

func (clint *CLINT) Store16(paddr, v uint64) error {
	return clint.store(paddr, v)
}

func (clint *CLINT) Store32(paddr, v uint64) error {
	return clint.store(paddr, v)
}

func (clint *CLINT) Store64(paddr, v uint64) error {
	return clint.store(paddr, v)
}
