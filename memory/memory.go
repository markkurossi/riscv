//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

// Package memory defines the interface to physical memory.
package memory

import (
	"encoding/binary"
)

var (
	bo = binary.LittleEndian
)

const (
	PageSize = 4096
)

// Avail tests if the page of the address addr has n bytes of data
// starting from the address (i.e. addr and addr+n are on the same
// page).
func Avail(addr, n uint64) bool {
	return addr&0xfff+n <= 0xfff
}

func Page(addr uint64) uint64 {
	return addr >> 12
}

func PageOffset(addr uint64) int {
	return int(addr & 0xfff)
}

// XXX should we remove {Load,Store}{16,64} from here and only provide
// byte-order neutral access to memory? Yes.
type Memory interface {
	AllocPage() (uint64, error)
	Page(num uint64) ([]byte, error)
	Data(addr uint64) ([]byte, error)

	Load(addr uint64, buf []byte) error
	Load8(addr uint64) (uint8, error)
	Load16(addr uint64) (uint16, error)
	Load64(addr uint64) (uint64, error)
	Store(addr uint64, data []byte) error
	Store8(addr, val uint64) error
	Store64(addr, val uint64) error
}
