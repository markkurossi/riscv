//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package dev

import (
	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/logger"
)

// Magic values.
const (
	PoweroffMagic = 0x706f6666 // poff
	RebootMagic   = 0x72656274 // rebt
	SuspendMagic  = 0x736c6570 // slep
)

// Syscon implements the System Controller device. The device
// implements the following functionality:
//
//	Offset	Magic			Description
//	------------------------------------
//	0		PoweroffMagic	shutdown CPU
//	4		RebootMagic		reboot CPU
type Syscon struct {
	logger.Logger
	Hart  isa.Hart
	Start uint64
	End   uint64
}

// NewSyscon creates a new syscon device.
func NewSyscon(hart isa.Hart, start, size uint64) *Syscon {
	return &Syscon{
		Logger: logger.Logger{
			Name:  "Syscon",
			Level: logger.Error,
		},
		Hart:  hart,
		Start: start,
		End:   start + size,
	}
}

// Halt implements MMIO.Halt.
func (syscon *Syscon) Halt() error {
	return nil
}

// Contains implements MMIO.Contains.
func (syscon *Syscon) Contains(paddr uint64) bool {
	return paddr >= syscon.Start && paddr < syscon.End
}

// Load8 implements MMIO.Load8.
func (syscon *Syscon) Load8(paddr uint64) (uint8, error) {
	syscon.Debugf("Syscon.Load8(%x)", paddr)
	return 0, nil
}

// Load16 implements MMIO.Load16.
func (syscon *Syscon) Load16(paddr uint64) (uint16, error) {
	syscon.Debugf("Syscon.Load16(%x)", paddr)
	return 0, nil
}

// Load32 implements MMIO.Load32.
func (syscon *Syscon) Load32(paddr uint64) (uint32, error) {
	syscon.Debugf("Syscon.Load32(%x)", paddr)
	return 0, nil
}

// Load64 implements MMIO.Load64.
func (syscon *Syscon) Load64(paddr uint64) (uint64, error) {
	syscon.Debugf("Syscon.Load64(%x)", paddr)
	return 0, nil
}

// Store8 implements MMIO.Store8.
func (syscon *Syscon) Store8(paddr uint64, v uint8) error {
	syscon.Debugf("Syscon.Store8(%x,%x)", paddr, v)
	syscon.Hart.Shutdown()
	return nil
}

// Store16 implements MMIO.Store16.
func (syscon *Syscon) Store16(paddr uint64, v uint16) error {
	syscon.Debugf("Syscon.Store16(%x,%x)", paddr, v)
	syscon.Hart.Shutdown()
	return nil
}

// Store32 implements MMIO.Store32.
func (syscon *Syscon) Store32(paddr uint64, v uint32) error {
	syscon.Debugf("Syscon.Store32(%x,%x)", paddr, v)

	ofs := paddr - syscon.Start
	switch ofs {
	case 0:
		if v != PoweroffMagic {
			syscon.Errorf("invalid poweroff magic %x, expected %x",
				v, PoweroffMagic)
		} else {
			syscon.Hart.Shutdown()
		}

	case 4:
		if v != RebootMagic {
			syscon.Errorf("invalid reboot magic %x, expected %x",
				v, RebootMagic)
		} else {
			syscon.Hart.Shutdown()
		}

	default:
		syscon.Errorf("invalid offset %x", ofs)
	}
	return nil
}

// Store64 implements MMIO.Store64.
func (syscon *Syscon) Store64(paddr uint64, v uint64) error {
	syscon.Debugf("Syscon.Store64(%x,%x)", paddr, v)
	syscon.Hart.Shutdown()
	return nil
}
