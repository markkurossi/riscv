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

func (mem *Memory) AllocPage() (uint64, error) {
	if mem.nextPage >= mem.numPages {
		return 0, ErrOutOfMemory
	}
	mem.nextPage++
	return uint64(mem.nextPage - 1), nil
}

func (mem *Memory) Page(num uint64) ([]byte, error) {
	if num >= uint64(mem.numPages) {
		return nil, ErrInvalidAddr
	}
	addr := num * PageSize
	return mem.RAM[addr : addr+PageSize], nil
}

func (mem *Memory) Load(addr uint64, buf []byte) error {
	if addr+uint64(len(buf)) > uint64(len(mem.RAM)) {
		return ErrInvalidAddr
	}
	copy(buf, mem.RAM[addr:])

	return nil
}

func (mem *Memory) Load8(addr uint64) (uint8, error) {
	if addr+1 > uint64(len(mem.RAM)) {
		return 0, ErrInvalidAddr
	}
	return mem.RAM[addr], nil
}

func (mem *Memory) Load16(addr uint64) (uint16, error) {
	if addr+2 > uint64(len(mem.RAM)) {
		return 0, ErrInvalidAddr
	}
	return bo.Uint16(mem.RAM[addr:]), nil
}

func (mem *Memory) Load64(addr uint64) (uint64, error) {
	if addr+8 > uint64(len(mem.RAM)) {
		return 0, ErrInvalidAddr
	}
	return bo.Uint64(mem.RAM[addr:]), nil
}

func (mem *Memory) Store(addr uint64, Data []byte) error {
	if addr+uint64(len(Data)) > uint64(len(mem.RAM)) {
		return ErrInvalidAddr
	}
	copy(mem.RAM[addr:], Data)

	return nil
}

func (mem *Memory) Store8(addr, val uint64) error {
	if addr+1 > uint64(len(mem.RAM)) {
		return ErrInvalidAddr
	}
	mem.RAM[addr] = uint8(val)

	return nil
}

func (mem *Memory) Store64(addr, val uint64) error {
	if addr+8 > uint64(len(mem.RAM)) {
		return ErrInvalidAddr
	}
	bo.PutUint64(mem.RAM[addr:], val)

	return nil
}
