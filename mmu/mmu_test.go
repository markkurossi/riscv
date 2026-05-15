//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package mmu

import (
	"fmt"
	"testing"

	"github.com/markkurossi/riscv/memory"
)

func makeMem() (*memory.Memory, Satp, uint64) {
	mem := memory.New(0, 10*memory.PageSize)

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

func makeTestMMU() (*MMU, uint64) {
	mem, satp, count := makeMem()
	return &MMU{
		satp: satp,
		Mem:  mem,
	}, count
}

func TestMapSV39(t *testing.T) {
	mmu, count := makeTestMMU()

	for i := uint64(1); i < count; i++ {
		vaddr := i * memory.PageSize
		paddr, err := mmu.Map(vaddr, AccessRead)
		if err != nil {
			t.Fatalf("MapSv39(%v): %v", vaddr, err)
		}
		if vaddr != paddr {
			t.Errorf("unexpected mapping from %v to %v", vaddr, paddr)
		}

		var ofs uint64 = 42

		paddr, err = mmu.Map(vaddr+ofs, AccessRead)
		if err != nil {
			t.Fatalf("MapSv39(%v): %v", vaddr+ofs, err)
		}
		if vaddr+ofs != paddr {
			t.Errorf("unexpected mapping from %v to %v", vaddr+ofs, paddr)
		}
	}
}

func BenchmarkMapSv39(b *testing.B) {
	mmu, _ := makeTestMMU()

	for b.Loop() {
		_, err := mmu.Map(4096, AccessRead)
		if err != nil {
			b.Fatal(err)
		}
	}
}
