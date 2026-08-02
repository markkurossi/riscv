//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package mmu

import (
	"fmt"

	"github.com/markkurossi/riscv/memory"
)

// UserCString reads a null-terminated C-string from virtual address
// vaddr.
func (mmu *MMU) UserCString(vaddr uint64) (string, error) {
	var data []byte

	for {
		paddr, err := mmu.Map(vaddr, AccessRead)
		if err != nil {
			return "", err
		}

		l := memory.PageSize - paddr%memory.PageSize
		for i := uint64(0); i < l; i++ {
			b := mmu.Mem.RAM[mmu.Mem.Offset(paddr+i)]
			if b == 0 {
				return string(data), nil
			}
			data = append(data, b)
		}

		vaddr += l
	}
}

// CopyFromUser copies data from virtual address vaddr to buf.
func (mmu *MMU) CopyFromUser(vaddr uint64, buf []byte) error {
	for len(buf) > 0 {
		l := memory.PageSize - vaddr%memory.PageSize
		if l > uint64(len(buf)) {
			l = uint64(len(buf))
		}
		paddr, err := mmu.Map(vaddr, AccessRead)
		if err != nil {
			return err
		}
		copy(buf[:l], mmu.Mem.RAM[mmu.Mem.Offset(paddr):])
		buf = buf[l:]
		vaddr += l
	}
	return nil
}

// CopyToUser copies data to virtual address vaddr.
func (mmu *MMU) CopyToUser(vaddr uint64, data []byte) error {
	for len(data) > 0 {
		l := memory.PageSize - vaddr%memory.PageSize
		if l > uint64(len(data)) {
			l = uint64(len(data))
		}
		paddr, err := mmu.Map(vaddr, AccessWrite)
		if err != nil {
			return err
		}
		if paddr < mmu.Mem.RAMBase {
			for i := uint64(0); i < l; i++ {
				err := mmu.MMIO.Store8(paddr+i, data[i])
				if err != nil {
					return err
				}
			}
		} else {
			copy(mmu.Mem.RAM[mmu.Mem.Offset(paddr):], data[:l])
		}
		data = data[l:]
		vaddr += l
	}
	return nil
}

// SetMapSv39 sets a page table mapping from vpage to ppage with
// flags.
func SetMapSv39(mem *memory.Memory, satp Satp, vpage, ppage uint64,
	flags PTEFlags) error {

	if satp.Mode() != SatpModeSv39 {
		return fmt.Errorf("invalid page-table mode: %v", satp.Mode())
	}

	flags |= PteV

	root := satp.PPN()
	base := root << 12

	// Walk levels 2-1.
	for level := 2; level > 0; level-- {
		idx := (vpage >> uint64(9*level)) & 0b111111111
		pteAddr := base + idx*8

		pte := PTE(memory.Uint64(mem.RAM, mem.Offset(pteAddr)))

		if pte.Valid() {
			if pte.Leaf() {
				return fmt.Errorf("superpage exists")
			}
			// Walk to the next level.
			base = pte.PPN() << 12
		} else {
			// Lazy allocation of next level page.
			newPage, err := mem.AllocPage()
			if err != nil {
				return err
			}
			newPageAddr := newPage << 12

			// Clear page.
			page, err := mem.Page(newPage)
			if err != nil {
				return err
			}
			clear(page)

			memory.PutUint64(mem.RAM, mem.Offset(pteAddr),
				uint64(MakePTE(newPage, PteV)))

			base = newPageAddr
		}
	}

	// Level 0.

	idx := vpage & 0b111111111
	pteAddr := base + idx*8

	pte := PTE(memory.Uint64(mem.RAM, mem.Offset(pteAddr)))
	if pte.Valid() {
		return fmt.Errorf("mapping already exists: %v", pte)
	}

	memory.PutUint64(mem.RAM, mem.Offset(pteAddr),
		uint64(MakePTE(ppage, flags)))

	return nil
}

// Dump prints page table mappings.
func (mmu *MMU) Dump() error {
	mode := mmu.satp.Mode()
	if mode == SatpModeBare {
		fmt.Printf("bare mode")
	} else if mode != SatpModeSv39 {
		return fmt.Errorf("unsupported mode %v", mode)
	}
	page := mmu.satp.PPN() << 12

	return mmu.dumpLevel(page, 0, 2)
}

func (mmu *MMU) dumpLevel(root, vpage uint64, level int) error {
	for i := uint64(0); i < 512; i++ {
		addr := root + i*8
		pte := PTE(memory.Uint64(mmu.Mem.RAM, mmu.Mem.Offset(addr)))
		if !pte.Valid() {
			continue
		}
		if level == 0 {
			fmt.Printf("pte: %v: %08x => %014x\n", pte, vpage+i, pte.PPN())
		} else {
			err := mmu.dumpLevel(pte.PPN()<<12, vpage|(i<<(9*level)), level-1)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
