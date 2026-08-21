//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package mmu

import (
	"encoding/binary"

	"github.com/markkurossi/riscv/isa"
)

var (
	_  MMIO = &SHM{}
	bo      = binary.LittleEndian
)

// SHM implements a memory-mapped shared memory region.
type SHM struct {
	Hart  isa.Hart
	Start uint64
	End   uint64
	Data  []byte
}

// Halt implements MMIO.Halt.
func (shm *SHM) Halt() error {
	return nil
}

// Contains implements MMIO.Contains.
func (shm *SHM) Contains(paddr, size uint64) bool {
	return shm.Start <= paddr && paddr+size <= shm.End
}

// Load8 implements MMIO.Load8.
func (shm *SHM) Load8(paddr uint64) (uint8, error) {
	return shm.Data[paddr-shm.Start], nil
}

// Load16 implements MMIO.Load16.
func (shm *SHM) Load16(paddr uint64) (uint16, error) {
	return bo.Uint16(shm.Data[paddr-shm.Start:]), nil
}

// Load32 implements MMIO.Load32.
func (shm *SHM) Load32(paddr uint64) (uint32, error) {
	return bo.Uint32(shm.Data[paddr-shm.Start:]), nil
}

// Load64 implements MMIO.Load64.
func (shm *SHM) Load64(paddr uint64) (uint64, error) {
	return bo.Uint64(shm.Data[paddr-shm.Start:]), nil
}

// Store8 implements MMIO.Store8.
func (shm *SHM) Store8(paddr uint64, v uint8) error {
	shm.Data[paddr-shm.Start] = v
	return nil
}

// Store16 implements MMIO.Store16.
func (shm *SHM) Store16(paddr uint64, v uint16) error {
	bo.PutUint16(shm.Data[paddr-shm.Start:], v)
	return nil
}

// Store32 implements MMIO.Store32.
func (shm *SHM) Store32(paddr uint64, v uint32) error {
	bo.PutUint32(shm.Data[paddr-shm.Start:], v)
	return nil
}

// Store64 implements MMIO.Store64.
func (shm *SHM) Store64(paddr uint64, v uint64) error {
	bo.PutUint64(shm.Data[paddr-shm.Start:], v)
	return nil
}
