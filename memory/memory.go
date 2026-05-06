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

// XXX should we remove {Load,Store}{16,64} from here and only provide
// byte-order neutral access to memory? Yes.
type Memory interface {
	AllocPage() (uint64, error)
	Load(addr uint64, buf []byte) error
	Load8(addr uint64) (uint8, error)
	Load16(addr uint64) (uint16, error)
	Load64(addr uint64) (uint64, error)
	Store(addr uint64, data []byte) error
	Store8(addr, val uint64) error
	Store64(addr, val uint64) error
}
