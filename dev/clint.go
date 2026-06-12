//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package dev

import (
	"log"

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

func (clint *CLINT) Halt() error {
	return nil
}

func (clint *CLINT) Contains(paddr uint64) bool {
	return paddr >= clint.Start && paddr < clint.End
}

// Tick explicitly syncs the CLINT internal counter with the main engine clock
// and re-evaluates the Machine Timer Interrupt (MTIP) line status.
func (clint *CLINT) Tick(now uint64) {
	clint.Mtime = now

	// Check if the timer threshold has been reached or passed
	if clint.Mtime >= clint.Mtimecmp {
		// Assert Machine Timer Interrupt Pending (Bit 7 of mip)
		clint.Hart.SetInterrupt(isa.IntMTIP)
	} else {
		// Clear the line if the deadline was moved forward into the future
		clint.Hart.ClearInterrupt(isa.IntMTIP)
	}
}

func (clint *CLINT) load(paddr uint64) (uint64, error) {
	if !clint.Contains(paddr) {
		return 0, clint.Hart.Trap(isa.CauseLoadPageFault, paddr, nil)
	}

	ofs := paddr - clint.Start
	var v uint64

	switch ofs {
	case ClintOfsMsip:
		v = clint.Msip

	case ClintOfsMtimecmp:
		v = clint.Mtimecmp

	case ClintOfsMtime:
		// Dynamic sync before read ensures OpenSBI gets the exact current time
		clint.Mtime = clint.Hart.Now()
		v = clint.Mtime

	default:
		// Support split 32-bit register window accesses commonly used by kernels
		if ofs >= ClintOfsMtimecmp && ofs < ClintOfsMtimecmp+8 {
			v = clint.Mtimecmp
			if ofs&4 != 0 {
				v >>= 32
			}
		} else if ofs >= ClintOfsMtime && ofs < ClintOfsMtime+8 {
			clint.Mtime = clint.Hart.Now()
			v = clint.Mtime
			if ofs&4 != 0 {
				v >>= 32
			}
		} else {
			log.Printf("CLINT: load: unknown register %x", ofs)
		}
	}

	log.Printf("CLINT.load(%x): %x", ofs, v)

	return v, nil
}

func (clint *CLINT) store(paddr, v uint64) error {
	if !clint.Contains(paddr) {
		return clint.Hart.Trap(isa.CauseStorePageFault, paddr, nil)
	}
	ofs := paddr - clint.Start

	log.Printf("CLINT.store(%x): %x", ofs, v)

	switch ofs {
	case ClintOfsMsip:
		clint.Msip = v
		if clint.Msip&1 != 0 {
			clint.Hart.SetInterrupt(isa.IntMSIP) // Machine Software Interrupt (Bit 3)
		} else {
			clint.Hart.ClearInterrupt(isa.IntMSIP)
		}

	case ClintOfsMtimecmp:
		clint.Mtimecmp = v
		clint.Tick(clint.Hart.Now())

	case ClintOfsMtime:
		clint.Mtime = v
		clint.Tick(clint.Mtime)

	default:
		// Handle partial 32-bit lower/upper sub-word register writes
		if ofs >= ClintOfsMtimecmp && ofs < ClintOfsMtimecmp+8 {
			clint.Mtimecmp = updateRegisterHalf(clint.Mtimecmp, ofs, v)
			clint.Tick(clint.Hart.Now())
		} else if ofs >= ClintOfsMtime && ofs < ClintOfsMtime+8 {
			clint.Mtime = updateRegisterHalf(clint.Mtime, ofs, v)
			clint.Tick(clint.Mtime)
		} else {
			log.Printf("CLINT: store: unknown register %x = %x", ofs, v)
		}
	}

	return nil
}

// Helper to merge 32-bit aligned MMIO writes into our 64-bit fields safely
func updateRegisterHalf(current uint64, offset uint64, val uint64) uint64 {
	if offset&4 != 0 {
		return (current & 0xffffffff) | (val << 32)
	}
	return (current & 0xffffffff00000000) | (val & 0xffffffff)
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
	return clint.load(paddr)
}

func (clint *CLINT) Store8(paddr uint64, v uint64) error {
	return clint.store(paddr, v)
}

func (clint *CLINT) Store16(paddr uint64, v uint64) error {
	return clint.store(paddr, v)
}

func (clint *CLINT) Store32(paddr uint64, v uint64) error {
	return clint.store(paddr, v)
}

func (clint *CLINT) Store64(paddr uint64, v uint64) error {
	return clint.store(paddr, v)
}
