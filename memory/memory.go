//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

// Package memory defines the interface to physical memory.
package memory

import (
	"fmt"
)

import (
	"encoding/binary"
	"errors"
)

var (
	bo = binary.LittleEndian

	// ErrInvalidAddr is returned when an invalid address request is
	// made.
	ErrInvalidAddr = errors.New("invalid address")

	// ErrOutOfMemory is returned when the system rans out of memory.
	ErrOutOfMemory = errors.New("out of memory")
)

const (
	// PageSize defines the system page size in bytes.
	PageSize = 4096

	// RAMBase defines the RAM base address.
	RAMBase = 0x80000000

	// InvalidPagenum defines an invalid page number. On Sv39, the
	// page indices are 12 bits and page numbers are 52 bits i.e. no
	// valid page number can have 64 bits set.
	InvalidPagenum = 0xffffffffffffffff
)

// Avail tests if the page of the address addr has n bytes of data
// starting from the address (i.e. addr and addr+n are on the same
// page).
func Avail(addr, n uint64) bool {
	return (addr&0xfff)+n <= 0x1000
}

// Page returns the page number from and address.
func Page(addr uint64) uint64 {
	return addr >> 12
}

// PageOffset returns the address' byte-offset within the page.
func PageOffset(addr uint64) int {
	return int(addr & 0xfff)
}

// Memory implements physical memory.
type Memory struct {
	RAM      []byte
	RAMBase  uint64
	RAMEnd   uint64
	BO       binary.ByteOrder
	numPages int
	nextPage int
}

// New creates a new memory of size bytes starting at the base
// address.
func New(base, size uint64) *Memory {
	if size%PageSize != 0 {
		panic("memory size is not multiple of page size")
	}

	return &Memory{
		RAM:      make([]byte, size),
		RAMBase:  base,
		RAMEnd:   base + size,
		BO:       bo,
		numPages: int(size / PageSize),
	}
}

// Contains tests if the memory contains the address.
func (mem *Memory) Contains(addr uint64) bool {
	return mem.RAMBase <= addr && addr < mem.RAMEnd
}

// Offset returns the address' offset in the Memory.RAM array.
func (mem *Memory) Offset(paddr uint64) uint64 {
	return paddr - mem.RAMBase
}

// AllocPage allocates a new page and returns its page number.
func (mem *Memory) AllocPage() (uint64, error) {
	if mem.nextPage >= mem.numPages {
		return 0, ErrOutOfMemory
	}
	mem.nextPage++
	return mem.RAMBase/PageSize + uint64(mem.nextPage-1), nil
}

// Page returns a slice to the memory page num.
func (mem *Memory) Page(num uint64) ([]byte, error) {
	RAMBasePage := mem.RAMBase / PageSize

	if num < RAMBasePage || num >= RAMBasePage+uint64(mem.numPages) {
		return nil, ErrInvalidAddr
	}
	addr := (num - RAMBasePage) * PageSize
	return mem.RAM[addr : addr+PageSize], nil
}

// Strings prints all ASCII characters from the memory.
func (mem *Memory) Strings() {
	fmt.Printf("*** strings ***\n")
	for _, b := range mem.RAM {
		switch b {
		case '\t', '\n', '\r':
			fmt.Printf("%c", b)

		default:
			if ' ' <= b && b <= '~' {
				fmt.Printf("%c", b)
			}
		}
	}
	fmt.Printf("\n*** strings ***\n")
}
