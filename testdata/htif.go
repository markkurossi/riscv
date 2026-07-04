//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package testdata

import (
	"encoding/binary"
	"fmt"
	"log"

	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/memory"
)

const (
	HTIFSize = 0x80
)

var (
	bo = binary.LittleEndian
)

// HTIF implements the Host Target Interface (HTIF) interface.
type HTIF struct {
	Hart  isa.Hart
	Start uint64
	End   uint64
	Mem   *memory.Memory

	ToHost   uint64
	FromHost uint64

	ExitStatus uint64
}

func NewHTIF(hart isa.Hart, start, size, to, from uint64,
	mem *memory.Memory) *HTIF {

	return &HTIF{
		Hart:     hart,
		Start:    start,
		End:      start + size,
		Mem:      mem,
		ToHost:   to,
		FromHost: from,
	}
}

// Contains implements Overlay.Contains
func (htif *HTIF) Contains(paddr uint64) bool {
	return paddr >= htif.Start && paddr < htif.End
}

// Load implements Overlay.Load.
func (htif *HTIF) Load(paddr uint64) error {
	log.Printf("HTIF.Load(%x)", paddr)
	return nil
}

// Store implements Overlay.Store.
func (htif *HTIF) Store(paddr uint64) error {
	ofs := paddr - htif.Start
	switch ofs {
	case 0x000:
		reg := bo.Uint64(htif.Mem.RAM[htif.Mem.Offset(paddr):])

		// Mark command completed.
		bo.PutUint64(htif.Mem.RAM[htif.Mem.Offset(paddr):], 0)

		devid := reg >> 56 & 0xff
		command := reg >> 48 & 0xff
		payload := reg & 0x00FFFFFFFFFFFFFF

		switch devid {
		case 0x00:
			switch command {
			case 0x00:
				htif.ExitStatus = payload
				htif.Hart.Shutdown()

			default:
				return fmt.Errorf("unknown dev command %02x", command)
			}

		default:
			return fmt.Errorf("unknown devid %02x", devid)
		}
	}

	return nil
}
