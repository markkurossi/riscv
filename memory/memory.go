//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

// Package memory defines the interface to physical memory.
package memory

import (
	"encoding/binary"
	"errors"
)

var (
	bo = binary.LittleEndian

	ErrInvalidAddr = errors.New("invalid address")
	ErrOutOfMemory = errors.New("out of memory")
)

const (
	PageSize = 4096
	RAMBase  = 0x80000000
)

// Avail tests if the page of the address addr has n bytes of data
// starting from the address (i.e. addr and addr+n are on the same
// page).
func Avail(addr, n uint64) bool {
	return (addr&0xfff)+n <= 0xfff
}

func Page(addr uint64) uint64 {
	return addr >> 12
}

func PageOffset(addr uint64) int {
	return int(addr & 0xfff)
}

type Memory struct {
	RAM      []byte
	RAMBase  uint64
	numPages int
	nextPage int
}

func NewMemory(ramSize int) *Memory {
	if ramSize&0xfff != 0 {
		panic("memory size is not multiple of page size")
	}

	return &Memory{
		RAM:      make([]byte, ramSize),
		RAMBase:  RAMBase,
		numPages: ramSize / PageSize,
	}
}

func (mem *Memory) Offset(paddr uint64) uint64 {
	return paddr - mem.RAMBase
}

func (mem *Memory) AllocPage() (uint64, error) {
	if mem.nextPage >= mem.numPages {
		return 0, ErrOutOfMemory
	}
	mem.nextPage++
	return mem.RAMBase/PageSize + uint64(mem.nextPage-1), nil
}

func (mem *Memory) Page(num uint64) ([]byte, error) {
	RAMBasePage := mem.RAMBase / PageSize

	if num < RAMBasePage || num >= RAMBasePage+uint64(mem.numPages) {
		return nil, ErrInvalidAddr
	}
	addr := (num - RAMBasePage) * PageSize
	return mem.RAM[addr : addr+PageSize], nil
}
