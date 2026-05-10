//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package kernel

import (
	"fmt"
	"os"

	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/mmu"
)

type VMA struct {
	Start uint64
	End   uint64

	Source *os.File
	Offset uint64

	Prot int
}

func (vma *VMA) PageTableFlags() mmu.PTEFlags {
	var flags mmu.PTEFlags

	if vma.Prot&mmu.AccessRead != 0 {
		flags |= mmu.PteR
	}
	if vma.Prot&mmu.AccessWrite != 0 {
		flags |= mmu.PteW
	}
	if vma.Prot&mmu.AccessRead != 0 {
		flags |= mmu.PteR
	}
	if vma.Prot&mmu.AccessExec != 0 {
		flags |= mmu.PteX
	}
	flags |= mmu.PteU | mmu.PteV

	return flags
}

func (vma *VMA) clone() *VMA {
	return &VMA{
		Start: vma.Start,
		End:   vma.End,
		Prot:  vma.Prot,
	}
}

func (vma *VMA) String() string {
	var prot string
	if vma.Prot&mmu.AccessExec != 0 {
		prot += "X"
	} else {
		prot += "."
	}
	if vma.Prot&mmu.AccessWrite != 0 {
		prot += "W"
	} else {
		prot += "."
	}
	if vma.Prot&mmu.AccessRead != 0 {
		prot += "R"
	} else {
		prot += "."
	}

	result := fmt.Sprintf("%10x:%10x %s", vma.Start, vma.End, prot)
	if vma.Source != nil {
		result += fmt.Sprintf(" <- %p @ %v", vma.Source, vma.Offset)
	}

	return result
}

func (vma *VMA) Contains(addr uint64) bool {
	return vma.Start <= addr && addr < vma.End
}

func (vma *VMA) Satisfies(cause uint64) bool {
	switch cause {
	case isa.CauseLoadPageFault:
		return vma.Prot&mmu.AccessRead != 0

	case isa.CauseStorePageFault:
		return vma.Prot&mmu.AccessWrite != 0

	case isa.CauseInstPageFault:
		return vma.Prot&mmu.AccessExec != 0

	default:
		return false
	}
}

func (vma *VMA) same(o *VMA) bool {
	return vma.Start == o.Start && vma.End == o.End
}

func (vma *VMA) overlaps(o *VMA) bool {
	return vma.Start < o.End && vma.End > o.Start
}

func (vma *VMA) covers(o *VMA) bool {
	return vma.Start <= o.Start && vma.End >= o.End
}

func (kern *Kernel) PrintVMA() {
	fmt.Printf("VMA:\n")
	for i, vma := range kern.VMA {
		fmt.Printf("%3d: %v\n", i, vma)
	}
}

func (kern *Kernel) AddVMA(start, end uint64, prot int,
	source *os.File, offset uint64) error {

	vma := &VMA{
		Start: start,
		End:   end,

		Source: source,
		Offset: offset,

		Prot: prot,
	}

	for i := 0; i < len(kern.VMA); {
		o := kern.VMA[i]

		if vma.same(o) {
			// Replace o.
			kern.VMA[i] = vma
			return kern.compactVMA()
		}
		if vma.covers(o) {
			// Remove o.
			copy(kern.VMA[i:], kern.VMA[i+1:])
			kern.VMA = kern.VMA[:len(kern.VMA)-1]
			continue
		}
		if o.covers(vma) {
			// New:      |nnn|
			// Old: |oooooooooooo|
			// => : |oooo|nnn|ooo|
			kern.VMA = append(kern.VMA, nil)
			kern.VMA = append(kern.VMA, nil)
			copy(kern.VMA[i+3:], kern.VMA[i+1:])

			tail := o.clone()
			tail.Start = vma.End

			o.End = vma.Start

			kern.VMA[i+1] = vma
			kern.VMA[i+2] = tail
			return kern.compactVMA()
		}

		if vma.overlaps(o) {
			// Split VMAs.
			if vma.Start < o.Start {
				// New: |nnnnnnnnnn|
				// Old:       |ooooooooo|
				// => : |nnnnnnnnnn|oooo|
				kern.VMA = append(kern.VMA, nil)
				copy(kern.VMA[i+1:], kern.VMA[i:])
				kern.VMA[i] = vma
				o.Start = vma.End
				return kern.compactVMA()
			} else {
				// New:      |nnnnnNNNNN|
				// Old: |ooooooooo|
				// => : |oooo|nnnn|, insert |NNNNN|
				kern.VMA = append(kern.VMA, nil)
				copy(kern.VMA[i+2:], kern.VMA[i+1:])

				tail := vma.clone()
				tail.End = o.End
				o.End = vma.Start
				vma.Start = tail.End
				kern.VMA[i+1] = tail
				i += 2
			}
		} else if vma.Start < o.Start {
			// Insert before o.
			kern.VMA = append(kern.VMA, nil)
			copy(kern.VMA[i+1:], kern.VMA[i:])
			kern.VMA[i] = vma
			return kern.compactVMA()
		} else {
			i++
		}
	}

	kern.VMA = append(kern.VMA, vma)
	return kern.compactVMA()
}

func (kern *Kernel) compactVMA() error {

	prev := kern.VMA[0]

	for i := 1; i < len(kern.VMA); {
		vma := kern.VMA[i]

		if vma.Start == vma.End {
			// Delete empty areas.
			copy(kern.VMA[i:], kern.VMA[i+1:])
			kern.VMA = kern.VMA[:len(kern.VMA)-1]
			continue
		}

		if vma.Start == prev.End && vma.Prot == prev.Prot &&
			vma.Source == prev.Source &&
			vma.Start-prev.Start == vma.Offset-prev.Offset {
			// Merge.
			prev.End = vma.End
			copy(kern.VMA[i:], kern.VMA[i+1:])
			kern.VMA = kern.VMA[:len(kern.VMA)-1]
			if vma.Source != nil {
				// XXX file refcounts
			}
		} else {
			prev = vma
			i++
		}
	}

	return kern.checkVMA()
}

func (kern *Kernel) checkVMA() error {
	var prev *VMA

	for _, vma := range kern.VMA {
		if prev != nil {
			if vma.Start >= vma.End {
				panic("empty VMA")
			}
			if vma.Start < prev.End {
				panic("not sorted")
			}
		}
		prev = vma
	}

	return nil
}
