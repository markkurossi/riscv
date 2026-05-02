//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package hw

import (
	"errors"
	"fmt"
	"testing"
)

var (
	_ Memory = &MemArray{}

	ErrInvalidAddr = errors.New("invalid address")
	ErrOutOfMemory = errors.New("out of memory")
)

type MemArray struct {
	Data     []byte
	NextPage uint64
}

func NewMemArray(numPages int) *MemArray {
	return &MemArray{
		Data: make([]byte, numPages*PageSize),
	}
}

func (mem *MemArray) AllocPage() (uint64, error) {
	if (mem.NextPage+1)*PageSize > uint64(len(mem.Data)) {
		return 0, ErrOutOfMemory
	}
	mem.NextPage++
	return mem.NextPage - 1, nil
}

func (mem *MemArray) Load64(addr uint64) (uint64, error) {
	if addr+8 > uint64(len(mem.Data)) {
		return 0, ErrInvalidAddr
	}
	return bo.Uint64(mem.Data[addr:]), nil
}

func (mem *MemArray) Store64(addr, val uint64) error {
	if addr+8 > uint64(len(mem.Data)) {
		return ErrInvalidAddr
	}
	bo.PutUint64(mem.Data[addr:], val)

	return nil
}

func makeMem() (Memory, Satp, uint64) {
	mem := NewMemArray(10)

	// Skip 0 page.
	_, err := mem.AllocPage()
	if err != nil {
		panic(err)
	}

	root, err := mem.AllocPage()
	if err != nil {
		panic(err)
	}
	satp := NewSATP(SatpModeSv39, root)

	for i := uint64(0); ; i++ {
		err := SetMapSv39(mem, satp, i, i, PteV|PteR)
		if err != nil {
			fmt.Printf("SetMapSv39: i=%v, err=%v\n", i, err)
			return mem, satp, i
		}
	}
}

func makeTestCPU() (*CPU, uint64) {
	mem, satp, count := makeMem()
	return &CPU{
		Satp:   satp,
		Memory: mem,
	}, count
}

func TestMapSV39(t *testing.T) {
	cpu, count := makeTestCPU()

	for i := uint64(0); i < count; i++ {
		vaddr := i * PageSize
		paddr, err := cpu.Map(vaddr, AccessRead)
		if err != nil {
			t.Fatalf("MapSv39(%v): %v", vaddr, err)
		}
		if vaddr != paddr {
			t.Errorf("unexpected mapping from %v to %v", vaddr, paddr)
		}

		var ofs uint64 = 42

		paddr, err = cpu.Map(vaddr+ofs, AccessRead)
		if err != nil {
			t.Fatalf("MapSv39(%v): %v", vaddr+ofs, err)
		}
		if vaddr+ofs != paddr {
			t.Errorf("unexpected mapping from %v to %v", vaddr+ofs, paddr)
		}
	}
}

func BenchmarkMapSv39(b *testing.B) {
	cpu, _ := makeTestCPU()

	for b.Loop() {
		_, err := cpu.Map(4096, AccessRead)
		if err != nil {
			b.Fatal(err)
		}
	}
}
