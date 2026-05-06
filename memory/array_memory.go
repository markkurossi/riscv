//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package memory

import (
	"errors"
)

var (
	_ Memory = &ArrayMemory{}

	ErrInvalidAddr = errors.New("invalid address")
	ErrOutOfMemory = errors.New("out of memory")
)

type ArrayMemory struct {
	Data     []byte
	NumPages int
	NextPage int
}

func NewArrayMemory(numPages int) *ArrayMemory {
	return &ArrayMemory{
		Data:     make([]byte, numPages*PageSize),
		NumPages: numPages,
	}
}

func (mem *ArrayMemory) AllocPage() (uint64, error) {
	if mem.NextPage >= mem.NumPages {
		return 0, ErrOutOfMemory
	}
	mem.NextPage++
	return uint64(mem.NextPage - 1), nil
}

func (mem *ArrayMemory) Page(num uint64) ([]byte, error) {
	if num >= uint64(mem.NumPages) {
		return nil, ErrInvalidAddr
	}
	addr := num * PageSize
	return mem.Data[addr : addr+PageSize], nil
}

func (mem *ArrayMemory) Load(addr uint64, buf []byte) error {
	if addr+uint64(len(buf)) > uint64(len(mem.Data)) {
		return ErrInvalidAddr
	}
	copy(buf, mem.Data[addr:])

	return nil
}

func (mem *ArrayMemory) Load8(addr uint64) (uint8, error) {
	if addr+1 > uint64(len(mem.Data)) {
		return 0, ErrInvalidAddr
	}
	return mem.Data[addr], nil
}

func (mem *ArrayMemory) Load16(addr uint64) (uint16, error) {
	if addr+2 > uint64(len(mem.Data)) {
		return 0, ErrInvalidAddr
	}
	return bo.Uint16(mem.Data[addr:]), nil
}

func (mem *ArrayMemory) Load64(addr uint64) (uint64, error) {
	if addr+8 > uint64(len(mem.Data)) {
		return 0, ErrInvalidAddr
	}
	return bo.Uint64(mem.Data[addr:]), nil
}

func (mem *ArrayMemory) Store(addr uint64, data []byte) error {
	if addr+uint64(len(data)) > uint64(len(mem.Data)) {
		return ErrInvalidAddr
	}
	copy(mem.Data[addr:], data)

	return nil
}

func (mem *ArrayMemory) Store8(addr, val uint64) error {
	if addr+1 > uint64(len(mem.Data)) {
		return ErrInvalidAddr
	}
	mem.Data[addr] = uint8(val)

	return nil
}

func (mem *ArrayMemory) Store64(addr, val uint64) error {
	if addr+8 > uint64(len(mem.Data)) {
		return ErrInvalidAddr
	}
	bo.PutUint64(mem.Data[addr:], val)

	return nil
}
