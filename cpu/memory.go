//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package cpu

import (
	"fmt"
	"strings"
)

const (
	AccessNone = 0
	AccessRead = 1 << iota
	AccessWrite
	AccessExec

	checkAccess = false
)

type MemoryX struct {
	Segments  []*Segment
	HeapStart uint64
	HeapEnd   uint64
	MmapStart uint64
	MmapEnd   uint64
}

type Segment struct {
	Start uint64
	End   uint64
	Data  []byte

	Read  bool
	Write bool
	Exec  bool
}

func (seg *Segment) String() string {
	var flags []string

	if seg.Read {
		flags = append(flags, "R")
	}
	if seg.Write {
		flags = append(flags, "W")
	}
	if seg.Exec {
		flags = append(flags, "X")
	}

	return fmt.Sprintf("Segment %x:%x [%v]",
		seg.Start, seg.End, strings.Join(flags, ""))
}

func (mem *MemoryX) Add(seg *Segment) {
	if seg.Start&0xfff != 0 {
		panic(fmt.Sprintf("segment start: %x\n", seg.Start))
	}

	for _, s := range mem.Segments {
		if seg.Start < s.End && seg.End > s.Start {
			fmt.Printf("MemoryX.Add: %v overlaps %v\n", seg, s)
		}
	}
	mem.Segments = append(mem.Segments, seg)
}

func (mem *MemoryX) Map(addr uint64, mode, size int) (*Segment, uint64, error) {
	for _, seg := range mem.Segments {
		if addr >= seg.Start && seg.End-uint64(size) >= addr {
			if checkAccess {
				if mode&AccessRead != 0 && !seg.Read {
					return nil, 0, fmt.Errorf("address %x not readable", addr)
				}
				if mode&AccessWrite != 0 && !seg.Write {
					return nil, 0, fmt.Errorf("address %x not writable", addr)
				}
				if mode&AccessExec != 0 && !seg.Exec {
					return nil, 0, fmt.Errorf("address %x not executable", addr)
				}
			}
			return seg, addr - seg.Start, nil
		}
	}
	return nil, 0, fmt.Errorf("invalid access %x", addr)
}

func (mem *MemoryX) Load8(addr uint64) (uint8, error) {
	seg, ofs, err := mem.Map(addr, AccessRead, 1)
	if err != nil {
		return 0, err
	}
	return seg.Data[ofs], nil
}

func (mem *MemoryX) Load16(addr uint64) (uint16, error) {
	seg, ofs, err := mem.Map(addr, AccessRead, 2)
	if err != nil {
		return 0, err
	}
	return bo.Uint16(seg.Data[ofs:]), nil
}

func (mem *MemoryX) Load32(addr uint64) (uint32, error) {
	seg, ofs, err := mem.Map(addr, AccessRead, 4)
	if err != nil {
		return 0, err
	}
	return bo.Uint32(seg.Data[ofs:]), nil
}

func (mem *MemoryX) Load64(addr uint64) (uint64, error) {
	seg, ofs, err := mem.Map(addr, AccessRead, 8)
	if err != nil {
		return 0, err
	}
	return bo.Uint64(seg.Data[ofs:]), nil
}

func (mem *MemoryX) LoadString(addr uint64) (string, error) {
	seg, ofs, err := mem.Map(addr, AccessRead, 1)
	if err != nil {
		return "", err
	}
	// XXX paged data.
	var end int
	for end = int(ofs); end < len(seg.Data) && seg.Data[end] != 0; end++ {
	}
	// XXX error if no '\0' found.
	return string(seg.Data[ofs:end]), nil
}

func (mem *MemoryX) Store8(addr, val uint64) error {
	seg, ofs, err := mem.Map(addr, AccessWrite, 1)
	if err != nil {
		return err
	}
	seg.Data[ofs] = uint8(val)

	return nil
}

func (mem *MemoryX) Store16(addr, val uint64) error {
	seg, ofs, err := mem.Map(addr, AccessWrite, 2)
	if err != nil {
		return err
	}
	bo.PutUint16(seg.Data[ofs:], uint16(val))

	return nil
}

func (mem *MemoryX) Store32(addr, val uint64) error {
	seg, ofs, err := mem.Map(addr, AccessWrite, 4)
	if err != nil {
		return err
	}
	bo.PutUint32(seg.Data[ofs:], uint32(val))

	return nil
}

func (mem *MemoryX) Store64(addr, val uint64) error {
	seg, ofs, err := mem.Map(addr, AccessWrite, 8)
	if err != nil {
		return err
	}
	bo.PutUint64(seg.Data[ofs:], val)

	return nil
}

func (mem *MemoryX) StoreData(addr uint64, data []byte) error {
	seg, ofs, err := mem.Map(addr, AccessWrite, len(data))
	if err != nil {
		return err
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
