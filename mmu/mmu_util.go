//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package mmu

import (
	"github.com/markkurossi/riscv/memory"
)

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
				err := mmu.ROM.Store8(paddr+i, uint64(data[i]))
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
