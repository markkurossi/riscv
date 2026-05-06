//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package cpu

import (
	"fmt"
	"testing"
)

func makeMem() (Memory, Satp, uint64) {
	mem := NewArrayMemory(10)

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

	for i := uint64(1); i < count; i++ {
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
