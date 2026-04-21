//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package emulator

import (
	"fmt"
)

type Memory struct {
	Segments []*Segment
}

type Segment struct {
	Start uint64
	End   uint64
	Data  []byte

	Read  bool
	Write bool
	Exec  bool
}

func (mem *Memory) Add(seg *Segment) {
	mem.Segments = append(mem.Segments, seg)
}

func (mem *Memory) Map(addr uint64, size int) (*Segment, uint64, error) {
	for _, seg := range mem.Segments {
		if addr >= seg.Start && seg.End-uint64(size) >= addr {
			return seg, addr - seg.Start, nil
		}
	}
	return nil, 0, fmt.Errorf("invalid access %x", addr)
}

func (mem *Memory) Load8(addr uint64) (uint8, error) {
	seg, ofs, err := mem.Map(addr, 1)
	if err != nil {
		return 0, err
	}
	if !seg.Read {
		return 0, fmt.Errorf("address %x not readable", addr)
	}
	return seg.Data[ofs], nil
}

func (mem *Memory) Load16(addr uint64) (uint16, error) {
	seg, ofs, err := mem.Map(addr, 2)
	if err != nil {
		return 0, err
	}
	if !seg.Read {
		return 0, fmt.Errorf("address %x not readable", addr)
	}
	return bo.Uint16(seg.Data[ofs:]), nil
}

func (mem *Memory) Load64(addr uint64) (uint64, error) {
	seg, ofs, err := mem.Map(addr, 8)
	if err != nil {
		return 0, err
	}
	if !seg.Read {
		return 0, fmt.Errorf("address %x not readable", addr)
	}
	return bo.Uint64(seg.Data[ofs:]), nil
}

func (mem *Memory) Store8(addr, val uint64) error {
	seg, ofs, err := mem.Map(addr, 1)
	if err != nil {
		return err
	}
	if !seg.Write {
		return fmt.Errorf("address %x not writable", addr)
	}
	seg.Data[ofs] = uint8(val)

	return nil
}

func (mem *Memory) Store32(addr, val uint64) error {
	seg, ofs, err := mem.Map(addr, 4)
	if err != nil {
		return err
	}
	if !seg.Write {
		return fmt.Errorf("address %x not writable", addr)
	}
	bo.PutUint32(seg.Data[ofs:], uint32(val))

	return nil
}

func (mem *Memory) Store64(addr, val uint64) error {
	seg, ofs, err := mem.Map(addr, 8)
	if err != nil {
		return err
	}
	if !seg.Write {
		return fmt.Errorf("address %x not writable", addr)
	}
	bo.PutUint64(seg.Data[ofs:], val)

	return nil
}

func (mem *Memory) StoreData(addr uint64, data []byte) error {
	seg, ofs, err := mem.Map(addr, len(data))
	if err != nil {
		return err
	}
	if !seg.Write {
		return fmt.Errorf("address %x not writable", addr)
	}
	copy(seg.Data[ofs:], data)

	return nil
}

func NewStack(addr, size uint64) *Segment {
	return &Segment{
		Start: addr - size,
		End:   addr,
		Data:  make([]byte, size),
		Read:  true,
		Write: true,
	}
}
