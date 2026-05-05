//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package cpu

import (
	"fmt"
)

const (
	AccessNone = 0
	AccessRead = 1 << iota
	AccessWrite
	AccessExec

	checkAccess = false
)

type Memory interface {
	AllocPage() (uint64, error)
	Load(addr uint64, buf []byte) error
	Load8(addr uint64) (uint8, error)
	Load16(addr uint64) (uint16, error)
	Load64(addr uint64) (uint64, error)
	Store(addr uint64, data []byte) error
	Store8(addr, val uint64) error
	Store64(addr, val uint64) error
}

const (
	PageSize = 4096
)

const (
	SatpModeBare = 0
	SatpModeSv39 = 8
	SatpModeSv48 = 9
	SatpModeSv57 = 10
	SatpModeSv64 = 11
)

type Satp uint64

func NewSATP(mode int, ppn uint64) Satp {
	return Satp(mode)<<60 | Satp(ppn&0x7ffffffffff)
}

func (satp Satp) Mode() int {
	return int(satp >> 60)
}

func (satp Satp) ASID() uint16 {
	return uint16(satp >> 44)
}

func (satp Satp) PPN() uint64 {
	return uint64(satp & 0x7ffffffffff)
}

//  63           54 53        28 27        19 18        10 9 8 7 6 5 4 3 2 1 0
// +---------------+------------+------------+------------+---+-+-+-+-+-+-+-+-+
// |    Reserved   | PPN[2]     | PPN[1]     | PPN[0]     |RSW|D|A|G|U|X|W|R|V|
// +---------------+------------+------------+------------+---+-+-+-+-+-+-+-+-+

type PTE uint64

const (
	PteV = 1 << iota // Valid
	PteR             // Readable
	PteW             // Writable
	PteX             // Executable
	PteU             // User
	PteG             // Global
	PteA             // Accessed
	PteD             // Dirty
)

func MakePTE(ppn uint64, flags uint64) PTE {
	return PTE(ppn<<10 | flags&0b1111111111)
}

func (pte PTE) String() string {
	var result string

	reserved := pte >> 54
	if reserved != 0 {
		result = fmt.Sprintf("\u2205=%x,", reserved)
	}
	result += fmt.Sprintf("%03x/%03x/%03x,", pte.PPN2(), pte.PPN1(), pte.PPN0())

	result += fmt.Sprintf("%02b,", pte>>8&0b11)

	if pte&PteD != 0 {
		result += "D"
	} else {
		result += "."
	}
	if pte&PteA != 0 {
		result += "A"
	} else {
		result += "."
	}
	if pte&PteG != 0 {
		result += "G"
	} else {
		result += "."
	}
	if pte&PteU != 0 {
		result += "U"
	} else {
		result += "."
	}
	if pte&PteX != 0 {
		result += "X"
	} else {
		result += "."
	}
	if pte&PteW != 0 {
		result += "W"
	} else {
		result += "."
	}
	if pte&PteR != 0 {
		result += "R"
	} else {
		result += "."
	}
	if pte&PteV != 0 {
		result += "V"
	} else {
		result += "."
	}

	return result
}

func (pte PTE) Valid() bool {
	return pte&PteV != 0
}

func (pte PTE) Readable() bool {
	return pte&PteR != 0
}

func (pte PTE) Writable() bool {
	return pte&PteW != 0
}

func (pte PTE) Executable() bool {
	return pte&PteX != 0
}

func (pte PTE) Leaf() bool {
	return (pte & (PteR | PteW | PteX)) != 0
}

func (pte PTE) PPN() uint64 {
	return uint64(pte >> 10)
}

func (pte PTE) PPN0() uint64 {
	return pte.PPN() & 0x1FF
}

func (pte PTE) PPN1() uint64 {
	return pte.PPN() >> 9 & 0x1FF
}

func (pte PTE) PPN2() uint64 {
	return pte.PPN() >> 18 & 0x1FF
}

// Virtual address:
//
//   63   39 38           30 29           21 20           12 11            0
//  +-------+---------------+---------------+---------------+---------------+
//  | Unused|   L2 Index    |   L1 Index    |   L0 Index    |    Offset     |
//  | (zero)|   (9 bits)    |   (9 bits)    |   (9 bits)    |   (12 bits)   |
//  +-------+---------------+---------------+---------------+---------------+

// >> 12 gives the page number
// >> 9*level gives the page-table index at level
//
// Since each entry is 8 bytes, we will multiply index by 8 i.e. <<3
// so we can do:
//
//	return (va >> (9 + 9*level)) & 0b111111111000
func index(va uint64, level int) uint64 {
	return (va >> (12 + 9*level)) & 0b111111111
}

type TLBEntry struct {
	Vaddr uint64
	Paddr uint64
}

func (cpu *CPU) Map(vaddr uint64, access int) (uint64, error) {
	switch cpu.Satp.Mode() {
	case SatpModeBare:
		return vaddr, nil

	case SatpModeSv39:

	default:
		return 0, fmt.Errorf("unsupported memory model %v", cpu.Satp.Mode())
	}

	if vaddr == 0 {
		// XXX Refactor this to function, used also in MapSv39.
		if access&AccessWrite != 0 {
			return 0, cpu.Trap(CauseStorePageFault, vaddr, nil)
		}
		if access&AccessExec != 0 {
			return 0, cpu.Trap(CauseInstPageFault, vaddr, nil)
		}

		// Default to load page fault.
		return 0, cpu.Trap(CauseLoadPageFault, vaddr, nil)
	}

	tlb := &cpu.TLB[vaddr%uint64(len(cpu.TLB))]
	if tlb.Vaddr == vaddr {
		return tlb.Paddr, nil
	}

	paddr, err := cpu.MapSv39(cpu.Satp.PPN(), vaddr, access)
	if err != nil {
		return 0, err
	}
	if true {
		tlb.Vaddr = vaddr
		tlb.Paddr = paddr
	}

	return paddr, nil
}

func (cpu *CPU) MapSv39(root, vaddr uint64, access int) (uint64, error) {
	base := root << 12

	for level := 2; level >= 0; level-- {
		idx := index(vaddr, level)
		pteAddr := base + idx*8

		v, err := cpu.Memory.Load64(pteAddr)
		if err != nil {
			return 0, cpu.Trap(CauseLoadPageFault, pteAddr, err)
		}

		pte := PTE(v)

		if !pte.Valid() {
			var err error
			if true {
				err = fmt.Errorf("PTE not valid: %v", pte)
			}

			if access&AccessWrite != 0 {
				return 0, cpu.Trap(CauseStorePageFault, vaddr, err)
			}
			if access&AccessExec != 0 {
				return 0, cpu.Trap(CauseInstPageFault, vaddr, err)
			}

			// Default to load page fault.
			return 0, cpu.Trap(CauseLoadPageFault, vaddr, err)
		}
		if pte.Leaf() {
			return cpu.mapLeaf(pte, vaddr, level, access)
		}

		// Walk to the next level.
		base = pte.PPN() << 12
	}

	return 0, cpu.Trap(CauseLoadPageFault, vaddr,
		fmt.Errorf("no leaf page found"))
}

func (cpu *CPU) mapLeaf(pte PTE, vaddr uint64, level, access int) (
	uint64, error) {

	// Check permissions.
	if access&AccessRead != 0 && !pte.Readable() {
		return 0, cpu.Trap(CauseLoadAccessFault, vaddr, nil)
	}
	if access&AccessWrite != 0 && !pte.Writable() {
		return 0, cpu.Trap(CauseStoreAccessFault, vaddr, nil)
	}
	if access&AccessExec != 0 && !pte.Executable() {
		return 0, cpu.Trap(CauseInstAccessFault, vaddr, nil)
	}

	// Enforce superpage alignment rules.

	var misaligned bool
	switch level {
	case 2: // 1 GiB page.
		misaligned = pte.PPN1() != 0 || pte.PPN0() != 0
	case 1: // 2 MiB page
		misaligned = pte.PPN0() != 0
	}
	if misaligned {
		if access&AccessRead != 0 {
			return 0, cpu.Trap(CauseLoadAddrMisaligned, vaddr, nil)
		}
		if access&AccessWrite != 0 {
			return 0, cpu.Trap(CauseStoreAddrMisaligned, vaddr, nil)
		}
		if access&AccessExec != 0 {
			return 0, cpu.Trap(CauseInstAddrMisaligned, vaddr, nil)
		}
	}

	// Construct physical address.

	var paddr uint64
	switch level {
	case 2: // 1 GiB
		paddr = pte.PPN2()<<30 | (vaddr & ((1 << 30) - 1))
	case 1: // 2 MiB
		paddr = pte.PPN2()<<30 | pte.PPN1()<<21 | (vaddr & ((1 << 21) - 1))
	case 0:
		paddr = pte.PPN()<<12 | (vaddr & 0xfff)
	default:
		panic("invalid level")
	}

	return paddr, nil
}

func (cpu *CPU) UserUint8(vaddr uint64) (uint8, error) {
	paddr, err := cpu.Map(vaddr, AccessRead)
	if err != nil {
		return 0, err
	}
	return cpu.Memory.Load8(paddr)
}

func (cpu *CPU) UserUint16(vaddr uint64) (uint16, error) {
	var buf [2]byte

	err := cpu.CopyFromUser(vaddr, buf[:])
	if err != nil {
		return 0, err
	}
	return bo.Uint16(buf[:]), nil
}

func (cpu *CPU) UserUint32(vaddr uint64) (uint32, error) {
	var buf [4]byte

	err := cpu.CopyFromUser(vaddr, buf[:])
	if err != nil {
		return 0, err
	}
	return bo.Uint32(buf[:]), nil
}

func (cpu *CPU) UserUint64(vaddr uint64) (uint64, error) {
	var buf [8]byte

	err := cpu.CopyFromUser(vaddr, buf[:])
	if err != nil {
		return 0, err
	}
	return bo.Uint64(buf[:]), nil
}

func (cpu *CPU) UserCString(vaddr uint64) (string, error) {
	var data []byte

	for {
		paddr, err := cpu.Map(vaddr, AccessRead)
		if err != nil {
			return "", err
		}

		l := PageSize - paddr%PageSize
		for i := uint64(0); i < l; i++ {
			b, err := cpu.Memory.Load8(paddr + i)
			if err != nil {
				return "", err
			}
			if b == 0 {
				return string(data), nil
			}
			data = append(data, b)
		}

		vaddr += l
	}
}

func (cpu *CPU) CopyFromUser(vaddr uint64, buf []byte) error {
	for len(buf) > 0 {
		l := PageSize - vaddr%PageSize
		if l > uint64(len(buf)) {
			l = uint64(len(buf))
		}

		paddr, err := cpu.Map(vaddr, AccessRead)
		if err != nil {
			return err
		}
		err = cpu.Memory.Load(paddr, buf[:l])
		if err != nil {
			return err
		}
		buf = buf[l:]
		vaddr += l
	}
	return nil
}

func (cpu *CPU) PutUserUint8(vaddr, v uint64) error {
	paddr, err := cpu.Map(vaddr, AccessWrite)
	if err != nil {
		return err
	}
	return cpu.Memory.Store8(paddr, v)
}

func (cpu *CPU) PutUserUint16(vaddr, v uint64) error {
	var buf [2]byte

	bo.PutUint16(buf[:], uint16(v))

	return cpu.CopyToUser(vaddr, buf[:])
}

func (cpu *CPU) PutUserUint32(vaddr, v uint64) error {
	var buf [4]byte

	bo.PutUint32(buf[:], uint32(v))

	return cpu.CopyToUser(vaddr, buf[:])
}

func (cpu *CPU) PutUserUint64(vaddr, v uint64) error {
	var buf [8]byte

	bo.PutUint64(buf[:], v)

	return cpu.CopyToUser(vaddr, buf[:])
}

func (cpu *CPU) CopyToUser(vaddr uint64, data []byte) error {
	for len(data) > 0 {
		l := PageSize - vaddr%PageSize
		if l > uint64(len(data)) {
			l = uint64(len(data))
		}

		paddr, err := cpu.Map(vaddr, AccessWrite)
		if err != nil {
			return err
		}
		err = cpu.Memory.Store(paddr, data[:l])
		if err != nil {
			return err
		}
		data = data[l:]
		vaddr += l
	}
	return nil
}

func SetMapSv39(mem Memory, satp Satp, vpage, ppage, flags uint64) error {
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

		v, err := mem.Load64(pteAddr)
		if err != nil {
			return err
		}
		pte := PTE(v)

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
			for i := uint64(0); i < PageSize; i += 8 {
				if err := mem.Store64(newPageAddr+i, 0); err != nil {
					return err
				}
			}
			err = mem.Store64(pteAddr, uint64(MakePTE(newPage, PteV)))
			if err != nil {
				return err
			}

			base = newPageAddr
		}
	}

	// Level 0.

	idx := vpage & 0b111111111
	pteAddr := base + idx*8

	v, err := mem.Load64(pteAddr)
	if err != nil {
		return err
	}
	pte := PTE(v)
	if pte.Valid() {
		return fmt.Errorf("mapping alredy exists: %v", pte)
	}

	return mem.Store64(pteAddr, uint64(MakePTE(ppage, flags)))
}
