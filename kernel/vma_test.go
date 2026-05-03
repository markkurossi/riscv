//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package kernel

import (
	"fmt"
	"testing"

	"github.com/markkurossi/riscv/cpu"
)

var vmaTests = []struct {
	Start uint64
	End   uint64
	Prot  int
}{
	{0x2000, 0x3000, cpu.AccessRead},
	{0x1000, 0x2000, cpu.AccessRead},
	{0x1000, 0x3000, cpu.AccessRead},
	{0x1000, 0x3000, cpu.AccessRead | cpu.AccessWrite},
	{0x1500, 0x2000, cpu.AccessExec},
	{0x0500, 0x1800, cpu.AccessRead},
	{0x2500, 0x5000, cpu.AccessRead},
}

func TestVMA(t *testing.T) {
	kern := new(Kernel)

	for idx, test := range vmaTests {
		err := kern.AddVMA(test.Start, test.End, test.Prot)
		if err != nil {
			t.Fatalf("VMA %v: failed to add: %v", idx, err)
		}
	}

	for idx, vma := range kern.VMA {
		fmt.Printf("vma%d: %v\n", idx, vma)
	}
}
