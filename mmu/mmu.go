//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

// Package mmu implements the memory management unit.
package mmu

import (
	"encoding/binary"
	"fmt"

	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/memory"
)

var (
	bo = binary.LittleEndian
)

const (
	AccessNone  = 0
	AccessRead  = int(PteR)
	AccessWrite = int(PteW)
	AccessExec  = int(PteX)
)

const (
	SatpModeBare = 0
	SatpModeSv39 = 8
	SatpModeSv48 = 9
	SatpModeSv57 = 10
	SatpModeSv64 = 11
)

type ROM interface {
	Contains(paddr uint64) bool
	Load8(padr uint64) (uint8, error)
	Load16(padr uint64) (uint16, error)
	Load32(padr uint64) (uint32, error)
	Load64(padr uint64) (uint64, error)
	Store8(padr, v uint64) error
	Store16(padr, v uint64) error
	Store32(padr, v uint64) error
	Store64(padr, v uint64) error
}

type MMU struct {
	satp Satp
	Hart isa.Hart
	Mem  *memory.Memory
	ROM  ROM

	Sum bool
	Mxr bool

	TLB [4096]TLBEntry
}

func (mmu *MMU) Satp() Satp {
	return mmu.satp
}

func (mmu *MMU) SetSatp(satp Satp) {
	mmu.satp = satp
	clear(mmu.TLB[:])
}

func (mmu *MMU) FlushTLB() {
	clear(mmu.TLB[:])
}

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

func (satp Satp) String() string {
	return fmt.Sprintf("mode=%v, ppn=%v", satp.Mode(), satp.PPN())
}

// PTE defines the page table entry.
//
//	 63           54 53        28 27     19 18     10 9 8 7 6 5 4 3 2 1 0
//	+---------------+------------+---------+---------+---+-+-+-+-+-+-+-+-+
//	|    Reserved   | PPN[2]     | PPN[1]  | PPN[0]  |RSW|D|A|G|U|X|W|R|V|
//	+---------------+------------+---------+---------+---+-+-+-+-+-+-+-+-+
type PTE uint64

type PTEFlags uint8

const (
	PteV PTEFlags = 1 << iota // Valid
	PteR                      // Readable
	PteW                      // Writable
	PteX                      // Executable
	PteU                      // User
	PteG                      // Global
	PteA                      // Accessed
	PteD                      // Dirty
)

func (flags PTEFlags) String() string {
	var result string

	if flags&PteD != 0 {
		result += "D"
	} else {
		result += "."
	}
	if flags&PteA != 0 {
		result += "A"
	} else {
		result += "."
	}
	if flags&PteG != 0 {
		result += "G"
	} else {
		result += "."
	}
	if flags&PteU != 0 {
		result += "U"
	} else {
		result += "."
	}
	if flags&PteX != 0 {
		result += "X"
	} else {
		result += "."
	}
	if flags&PteW != 0 {
		result += "W"
	} else {
		result += "."
	}
	if flags&PteR != 0 {
		result += "R"
	} else {
		result += "."
	}
	if flags&PteV != 0 {
		result += "V"
	} else {
		result += "."
	}

	return result
}

func (flags PTEFlags) Valid() bool {
	return flags&PteV != 0
}

func (flags PTEFlags) Readable() bool {
	return flags&PteR != 0
}

func (flags PTEFlags) Writable() bool {
	return flags&PteW != 0
}

func (flags PTEFlags) Executable() bool {
	return flags&PteX != 0
}

func (flags PTEFlags) User() bool {
	return flags&PteU != 0
}

func (flags PTEFlags) CanAccess(access int) (bool, uint64) {
	if int(flags)&access == access {
		return true, 0
	}
	if access&AccessWrite != 0 && !flags.Writable() {
		return false, isa.CauseStorePageFault
	}
	if access&AccessExec != 0 && !flags.Executable() {
		return false, isa.CauseInstPageFault
	}
	return false, isa.CauseLoadPageFault
}

func MakePTE(ppn uint64, flags PTEFlags) PTE {
	return PTE(ppn<<10 | uint64(flags&0b11111111))
}

func (pte PTE) String() string {
	var result string

	result += fmt.Sprintf("%011x/%03x/%03x,",
		pte.PPN2(), pte.PPN1(), pte.PPN0())

	result += fmt.Sprintf("%02b,", pte>>8&0b11)

	result += pte.Flags().String()
	return result
}

func (pte PTE) Flags() PTEFlags {
	return PTEFlags(pte & 0b11111111)
}

func (pte PTE) Valid() bool {
	return pte.Flags().Valid()
}

func (pte PTE) Readable() bool {
	return pte.Flags()&PteR != 0
}

func (pte PTE) Writable() bool {
	return pte.Flags()&PteW != 0
}

func (pte PTE) Executable() bool {
	return pte.Flags()&PteX != 0
}

func (pte PTE) User() bool {
	return pte.Flags()&PteU != 0
}

func (pte PTE) Leaf() bool {
	return (pte.Flags() & (PteR | PteW | PteX)) != 0
}

func (pte PTE) PPN() uint64 {
	return uint64(pte) >> 10
}

func (pte PTE) PPN0() uint64 {
	return pte.PPN() & 0x1FF
}

func (pte PTE) PPN1() uint64 {
	return pte.PPN() >> 9 & 0x1FF
}

func (pte PTE) PPN2() uint64 {
	return pte.PPN() >> 18
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
	return uint64((va >> (12 + (9 * level))) & 0x1FF)
}

type TLBEntry struct {
	VPN        uint64
	Page       uint64
	Flags      PTEFlags
	OffsetMask uint32
}

func (mmu *MMU) Map(vaddr uint64, access int) (uint64, error) {
	if mmu.satp.Mode() == SatpModeBare {
		return vaddr, nil
	}
	if mmu.Hart.Mode() == isa.ModeM {
		return vaddr, nil
	}

	vpn := vaddr >> 12
	tlb := &mmu.TLB[vpn&0xfff]

	if tlb.VPN == vpn && tlb.Flags&PteV != 0 {
		if int(tlb.Flags)&access == access {
			return tlb.Page | (vaddr & uint64(tlb.OffsetMask)), nil
		}
	}

	addr, err := mmu.mapSlow(vaddr, vpn, access)
	if err != nil {
		if false {
			// XXX kludge checks for ROM and low RAM addresses???
			if vaddr < mmu.Mem.RAMBase ||
				(mmu.Mem.RAMBase <= vaddr &&
					vaddr <= mmu.Mem.RAMBase+0x20000000) {
				return vaddr, nil
			}
		}
		return 0, err
	}
	if false {
		fmt.Printf("%016x => %016x\n", vaddr, addr)
	}
	return addr, nil
}

func (mmu *MMU) mapSlow(vaddr, vpn uint64, access int) (uint64, error) {
	page, flags, level, err := mmu.MapSv39(mmu.satp.PPN(), vaddr, access)
	if err != nil {
		fmt.Printf("mmu.MapSv39 failed: %v\n", err)
		return 0, err
	}

	tlb := &mmu.TLB[vpn&0xfff]

	tlb.VPN = vpn
	tlb.Page = page
	tlb.Flags = flags | PteV

	switch level {
	case 2:
		tlb.OffsetMask = (1<<30 - 1)
	case 1:
		tlb.OffsetMask = (1<<21 - 1)
	case 0:
		tlb.OffsetMask = (1<<12 - 1)
	default:
		panic("invalid level")
	}

	return page | (vaddr & uint64(tlb.OffsetMask)), nil
}

func (mmu *MMU) MapSv39(root, vaddr uint64, access int) (
	uint64, PTEFlags, int, error) {

	base := root << 12

	fmt.Printf("MMU.MapSv39: root=%x => base=%x, vaddr=%x\n",
		root, base, vaddr)

	for level := 2; level >= 0; level-- {
		idx := index(vaddr, level)
		pteAddr := base + idx*8

		pte := PTE(bo.Uint64(mmu.Mem.RAM[mmu.Mem.Offset(pteAddr):]))

		if !pte.Valid() {
			var err error
			if true {
				err = fmt.Errorf("PTE not valid: %v", pte)
			}

			if access&AccessWrite != 0 {
				return 0, 0, 0,
					mmu.Hart.Trap(isa.CauseStorePageFault, vaddr, err)
			}
			if access&AccessExec != 0 {
				return 0, 0, 0,
					mmu.Hart.Trap(isa.CauseInstPageFault, vaddr, err)
			}

			// Default to load page fault.
			return 0, 0, 0,
				mmu.Hart.Trap(isa.CauseLoadPageFault, vaddr, err)
		}
		if pte.Leaf() {
			return mmu.mapLeaf(pte, vaddr, level, access)
		}

		// Walk to the next level.
		base = pte.PPN() << 12
	}

	return 0, 0, 0, mmu.Hart.Trap(isa.CauseLoadPageFault, vaddr,
		fmt.Errorf("no leaf page found"))
}

type AccessContext struct {
	Addr   uint64
	PTE    PTE
	Access int
	SUM    bool
	MXR    bool
	Desc   string
}

func (ac *AccessContext) Error() string {
	result := fmt.Sprintf("context: addr=%x, pte=%v, access=%d, sum=%v, mxr=%v",
		ac.Addr, ac.PTE, ac.Access, ac.SUM, ac.MXR)
	if len(ac.Desc) != 0 {
		result += ", desc=" + ac.Desc
	}
	return result
}

func (ac *AccessContext) WithDesc(desc string) *AccessContext {
	ac.Desc = desc
	return ac
}

func (mmu *MMU) mapLeaf(pte PTE, vaddr uint64, level, access int) (
	uint64, PTEFlags, int, error) {

	ac := &AccessContext{
		Addr:   vaddr,
		PTE:    pte,
		Access: access,
		SUM:    mmu.Sum,
		MXR:    mmu.Mxr,
	}

	readable := pte.Readable()
	if mmu.Mxr && pte.Executable() {
		// MXR overrides read constraints if executable is active
		readable = true
	}

	// Check permissions.
	if access&AccessRead != 0 && !readable {
		return 0, 0, 0, mmu.Hart.Trap(isa.CauseLoadPageFault, vaddr, ac)
	}
	if access&AccessWrite != 0 && !pte.Writable() {
		return 0, 0, 0, mmu.Hart.Trap(isa.CauseStorePageFault, vaddr, ac)
	}
	if access&AccessExec != 0 && !pte.Executable() {
		return 0, 0, 0, mmu.Hart.Trap(isa.CauseInstPageFault, vaddr, ac)
	}

	isUserPage := pte.User()

	if mmu.Hart.Mode() == isa.ModeU {
		// User mode CANNOT access kernel pages (PteU == 0)
		if !isUserPage {
			if access&AccessExec != 0 {
				return 0, 0, 0, mmu.Hart.Trap(isa.CauseInstPageFault,
					vaddr, ac.WithDesc("U-mode"))
			}
			if access&AccessWrite != 0 {
				return 0, 0, 0, mmu.Hart.Trap(isa.CauseStorePageFault,
					vaddr, ac.WithDesc("U-mode"))
			}
			return 0, 0, 0, mmu.Hart.Trap(isa.CauseLoadPageFault,
				vaddr, ac.WithDesc("U-mode"))
		}
	} else if mmu.Hart.Mode() == isa.ModeS {
		// Supervisor mode accessing a User Page (PteU == 1)
		if isUserPage {
			// Rule A: Supervisor mode can NEVER execute code from a User page
			if access&AccessExec != 0 {
				return 0, 0, 0, mmu.Hart.Trap(isa.CauseInstPageFault,
					vaddr, ac.WithDesc("S-mode exec user page"))
			}
			// Rule B: Supervisor data access is ONLY allowed if
			// sstatus.SUM == 1
			if !mmu.Sum {
				if access&AccessWrite != 0 {
					return 0, 0, 0, mmu.Hart.Trap(isa.CauseStorePageFault,
						vaddr, ac.WithDesc("S-mode user page"))
				}
				return 0, 0, 0, mmu.Hart.Trap(isa.CauseLoadPageFault,
					vaddr, ac.WithDesc("S-mode user page"))
			}
		}
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
		if access&AccessWrite != 0 {
			return 0, 0, 0,
				mmu.Hart.Trap(isa.CauseStoreAddrMisaligned, vaddr, ac)
		}
		if access&AccessExec != 0 {
			return 0, 0, 0,
				mmu.Hart.Trap(isa.CauseInstAddrMisaligned, vaddr, ac)
		}
		// Default to load fault.
		return 0, 0, 0,
			mmu.Hart.Trap(isa.CauseLoadAddrMisaligned, vaddr, ac)
	}

	var page uint64
	switch level {
	case 2: // 1 GiB
		page = pte.PPN2() << 30
	case 1: // 2 MiB
		page = pte.PPN2()<<30 | pte.PPN1()<<21
	case 0:
		page = pte.PPN() << 12
	default:
		panic("invalid level")
	}

	return page, pte.Flags(), level, nil
}

func (mmu *MMU) Load8(vaddr uint64) (uint8, error) {
	paddr, err := mmu.Map(vaddr, AccessRead)
	if err != nil {
		return 0, err
	}
	if paddr < mmu.Mem.RAMBase {
		return mmu.ROM.Load8(paddr)
	}
	return mmu.Mem.RAM[mmu.Mem.Offset(paddr)], nil
}

func (mmu *MMU) Load16(vaddr uint64) (uint16, error) {
	if memory.Avail(vaddr, 2) {
		paddr, err := mmu.Map(vaddr, AccessRead)
		if err != nil {
			return 0, err
		}
		if paddr < mmu.Mem.RAMBase {
			return mmu.ROM.Load16(paddr)
		}
		return bo.Uint16(mmu.Mem.RAM[mmu.Mem.Offset(paddr):]), nil
	}

	var page uint64
	var result uint16
	var buf []byte

	for i := 0; i < 2; i++ {
		if memory.Page(vaddr) != page {
			paddr, err := mmu.Map(vaddr, AccessRead)
			if err != nil {
				return 0, err
			}
			if paddr < mmu.Mem.RAMBase {
				return mmu.ROM.Load16(paddr)
			}
			buf = mmu.Mem.RAM[mmu.Mem.Offset(paddr):]
			page = memory.Page(vaddr)
		}
		result |= uint16(buf[0]) << (i * 8)
		buf = buf[1:]
		vaddr++
	}

	return result, nil
}

func (mmu *MMU) Load32(vaddr uint64) (uint32, error) {
	if memory.Avail(vaddr, 4) {
		paddr, err := mmu.Map(vaddr, AccessRead)
		if err != nil {
			return 0, err
		}
		if paddr < mmu.Mem.RAMBase {
			return mmu.ROM.Load32(paddr)
		}
		return bo.Uint32(mmu.Mem.RAM[mmu.Mem.Offset(paddr):]), nil
	}

	var page uint64
	var result uint32
	var buf []byte

	for i := 0; i < 4; i++ {
		if memory.Page(vaddr) != page {
			paddr, err := mmu.Map(vaddr, AccessRead)
			if err != nil {
				return 0, err
			}
			if paddr < mmu.Mem.RAMBase {
				return mmu.ROM.Load32(paddr)
			}
			buf = mmu.Mem.RAM[mmu.Mem.Offset(paddr):]
			page = memory.Page(vaddr)
		}
		result |= uint32(buf[0]) << (i * 8)
		buf = buf[1:]
		vaddr++
	}

	return result, nil
}

func (mmu *MMU) Load64(vaddr uint64) (uint64, error) {
	if memory.Avail(vaddr, 8) {
		paddr, err := mmu.Map(vaddr, AccessRead)
		if err != nil {
			return 0, err
		}
		if paddr < mmu.Mem.RAMBase {
			return mmu.ROM.Load64(paddr)
		}
		if paddr-mmu.Mem.RAMBase >= uint64(len(mmu.Mem.RAM)) && true {
			return 0, mmu.Hart.Trap(isa.CauseLoadPageFault, vaddr,
				fmt.Errorf("mapped page out of range: vaddr=%x, paddr=%x",
					vaddr, paddr))
		}
		return bo.Uint64(mmu.Mem.RAM[mmu.Mem.Offset(paddr):]), nil
	}

	var page uint64
	var result uint64
	var buf []byte

	for i := 0; i < 8; i++ {
		if memory.Page(vaddr) != page {
			paddr, err := mmu.Map(vaddr, AccessRead)
			if err != nil {
				return 0, err
			}
			if paddr < mmu.Mem.RAMBase {
				return mmu.ROM.Load64(paddr)
			}
			buf = mmu.Mem.RAM[mmu.Mem.Offset(paddr):]
			page = memory.Page(vaddr)
		}
		result |= uint64(buf[0]) << (i * 8)
		buf = buf[1:]
		vaddr++
	}

	return result, nil
}

func (mmu *MMU) Store8(vaddr, v uint64) error {
	paddr, err := mmu.Map(vaddr, AccessWrite)
	if err != nil {
		return err
	}
	if paddr < mmu.Mem.RAMBase {
		return mmu.ROM.Store8(paddr, v)
	}
	mmu.Mem.RAM[mmu.Mem.Offset(paddr)] = byte(v)
	return nil
}

func (mmu *MMU) Store16(vaddr, v uint64) error {
	if memory.Avail(vaddr, 2) {
		paddr, err := mmu.Map(vaddr, AccessWrite)
		if err != nil {
			return err
		}
		if paddr < mmu.Mem.RAMBase {
			return mmu.ROM.Store16(paddr, v)
		}
		bo.PutUint16(mmu.Mem.RAM[mmu.Mem.Offset(paddr):], uint16(v))
		return nil

	}
	var buf [2]byte

	bo.PutUint16(buf[:], uint16(v))

	return mmu.CopyToUser(vaddr, buf[:])
}

func (mmu *MMU) Store32(vaddr, v uint64) error {
	if memory.Avail(vaddr, 4) {
		paddr, err := mmu.Map(vaddr, AccessWrite)
		if err != nil {
			return err
		}
		if paddr < mmu.Mem.RAMBase {
			return mmu.ROM.Store32(paddr, v)
		}
		bo.PutUint32(mmu.Mem.RAM[mmu.Mem.Offset(paddr):], uint32(v))
		return nil
	}
	var buf [4]byte

	bo.PutUint32(buf[:], uint32(v))

	return mmu.CopyToUser(vaddr, buf[:])
}

func (mmu *MMU) Store64(vaddr, v uint64) error {
	if memory.Avail(vaddr, 8) {
		paddr, err := mmu.Map(vaddr, AccessWrite)
		if err != nil {
			return err
		}
		if paddr < mmu.Mem.RAMBase {
			return mmu.ROM.Store64(paddr, v)
		}
		bo.PutUint64(mmu.Mem.RAM[mmu.Mem.Offset(paddr):], v)
		return nil
	}
	var buf [8]byte

	bo.PutUint64(buf[:], v)

	return mmu.CopyToUser(vaddr, buf[:])
}

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

		pte := PTE(bo.Uint64(mem.RAM[mem.Offset(pteAddr):]))

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

			bo.PutUint64(mem.RAM[mem.Offset(pteAddr):],
				uint64(MakePTE(newPage, PteV)))

			base = newPageAddr
		}
	}

	// Level 0.

	idx := vpage & 0b111111111
	pteAddr := base + idx*8

	pte := PTE(bo.Uint64(mem.RAM[mem.Offset(pteAddr):]))
	if pte.Valid() {
		return fmt.Errorf("mapping already exists: %v", pte)
	}

	bo.PutUint64(mem.RAM[mem.Offset(pteAddr):], uint64(MakePTE(ppage, flags)))

	return nil
}
