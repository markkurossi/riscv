//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

// Package mmu implements the memory management unit.
package mmu

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"

	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/memory"
)

var (
	bo = binary.LittleEndian
)

const (
	debugMMU = false
)

// Memory access flags.
const (
	AccessNone  = 0
	AccessRead  = int(PteR)
	AccessWrite = int(PteW)
	AccessExec  = int(PteX)
)

// Supervisor Address Translation and Protection (SATP) modes.
const (
	SatpModeBare = 0
	SatpModeSv39 = 8
	SatpModeSv48 = 9
	SatpModeSv57 = 10
	SatpModeSv64 = 11
)

// MMIO implements a memory-mapped device.
type MMIO interface {
	Halt() error
	Contains(paddr uint64) bool
	Load8(paddr uint64) (uint8, error)
	Load16(paddr uint64) (uint16, error)
	Load32(paddr uint64) (uint32, error)
	Load64(paddr uint64) (uint64, error)
	Store8(paddr uint64, v uint8) error
	Store16(paddr uint64, v uint16) error
	Store32(paddr uint64, v uint32) error
	Store64(paddr uint64, v uint64) error
}

// MMU implements the memory management unit.
type MMU struct {
	satp Satp
	Hart isa.Hart
	Mem  *memory.Memory
	MMIO MMIO

	TLB [4096]TLBEntry
}

// Satp returns the MMU's current Supervisor Address Translation and
// Protection (SATP) value.
func (mmu *MMU) Satp() Satp {
	return mmu.satp
}

// SetSatp sets the satp configuration. All subsequent address
// translations will use the satp accordingly.
func (mmu *MMU) SetSatp(satp Satp) {
	mmu.satp = satp
	clear(mmu.TLB[:])
}

// FlushTLB flushes the MMU's TLB.
func (mmu *MMU) FlushTLB() {
	clear(mmu.TLB[:])
}

// Satp defines the Supervisor Address Translation and Protection
// configuration.
type Satp uint64

// NewSATP creates a new satp with mode and page table root physical
// page number (PPN).
func NewSATP(mode int, ppn uint64) Satp {
	return Satp(mode)<<60 | Satp(ppn&0x7ffffffffff)
}

// Mode returns the satp mode.
func (satp Satp) Mode() int {
	return int(satp >> 60)
}

// ASID returns the satp Address Space ID (ASID).
func (satp Satp) ASID() uint16 {
	return uint16(satp >> 44)
}

// PPN returns the satp PPN.
func (satp Satp) PPN() uint64 {
	return uint64(satp & 0x7ffffffffff)
}

func (satp Satp) String() string {
	return fmt.Sprintf("mode=%v, asid=%x, ppn=%x",
		satp.Mode(), satp.ASID(), satp.PPN())
}

// PTE defines the page table entry.
//
//	 63           54 53        28 27     19 18     10 9 8 7 6 5 4 3 2 1 0
//	+---------------+------------+---------+---------+---+-+-+-+-+-+-+-+-+
//	|    Reserved   | PPN[2]     | PPN[1]  | PPN[0]  |RSW|D|A|G|U|X|W|R|V|
//	+---------------+------------+---------+---------+---+-+-+-+-+-+-+-+-+
type PTE uint64

// PTEFlags define page table entry flags.
type PTEFlags uint8

// Page table entry flags.
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

// Valid tests if the entry is valid.
func (flags PTEFlags) Valid() bool {
	return flags&PteV != 0
}

// Readable tests if the page is readable.
func (flags PTEFlags) Readable() bool {
	return flags&PteR != 0
}

// Writable tests if the page is writable.
func (flags PTEFlags) Writable() bool {
	return flags&PteW != 0
}

// Executable tests if the page is executable.
func (flags PTEFlags) Executable() bool {
	return flags&PteX != 0
}

// User tests if the page is for user-mode.
func (flags PTEFlags) User() bool {
	return flags&PteU != 0
}

// Accessed tests if the page has been accessed.
func (flags PTEFlags) Accessed() bool {
	return flags&PteA != 0
}

// Dirty tests if the page has been modified.
func (flags PTEFlags) Dirty() bool {
	return flags&PteD != 0
}

// CanAccess tests if the page can be accessed with the mode access.
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

// MakePTE creates a new pate table entry for the physical page number
// ppn and flags.
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

// Flags return the page table entry flags.
func (pte PTE) Flags() PTEFlags {
	return PTEFlags(pte & 0b11111111)
}

// SetFlags set the page table entry flags.
func (pte PTE) SetFlags(flags PTEFlags) {
	pte &^= 0b11111111
	pte |= PTE(flags)
}

// Valid tests if the entry is valid.
func (pte PTE) Valid() bool {
	return pte.Flags().Valid()
}

// Readable tests if the page is readable.
func (pte PTE) Readable() bool {
	return pte.Flags()&PteR != 0
}

// Writable tests if the page is writable.
func (pte PTE) Writable() bool {
	return pte.Flags()&PteW != 0
}

// Executable tests if the page is executable.
func (pte PTE) Executable() bool {
	return pte.Flags()&PteX != 0
}

// User tests if the page is for user-mode.
func (pte PTE) User() bool {
	return pte.Flags()&PteU != 0
}

// Leaf tests if the page table entry is a leaf entry.
func (pte PTE) Leaf() bool {
	return (pte.Flags() & (PteR | PteW | PteX)) != 0
}

// PPN returns the entry's physical page number.
func (pte PTE) PPN() uint64 {
	return uint64(pte) >> 10
}

// PPN0 returns the entry's physical page number level 0.
func (pte PTE) PPN0() uint64 {
	return pte.PPN() & 0x1FF
}

// PPN1 returns the entry's physical page number level 1.
func (pte PTE) PPN1() uint64 {
	return pte.PPN() >> 9 & 0x1FF
}

// PPN2 returns the entry's physical page number level 2.
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

// TLBEntry defines a Translation Lookaside Buffer (TLB) entry.
type TLBEntry struct {
	VPN        uint64
	Page       uint64
	Flags      PTEFlags
	OffsetMask uint32
	UserMode   bool
}

// Clear clears the TLB entry.
func (te *TLBEntry) Clear() {
	te.VPN = 0
	te.Page = 0
	te.Flags = 0
	te.OffsetMask = 0
	te.UserMode = false
}

// Map maps the virtual address to physical address. The map enforces
// the access flags and generates memory access faults if the page
// table mapping does not allow the specified page access.
func (mmu *MMU) Map(vaddr uint64, access int) (uint64, error) {
	if mmu.satp.Mode() == SatpModeBare {
		return vaddr, nil
	}
	mode := mmu.Hart.Mode()
	if mode == isa.ModeM {
		return vaddr, nil
	}

	vpn := vaddr >> 12
	tlb := &mmu.TLB[vpn&0xfff]

	if tlb.VPN == vpn && tlb.Flags&PteV != 0 &&
		tlb.UserMode == (mode == isa.ModeU) {

		var dirtyOk bool
		if access&AccessWrite != 0 {
			dirtyOk = tlb.Flags.Dirty()
		} else {
			dirtyOk = true
		}

		if dirtyOk && int(tlb.Flags)&access == access {
			return tlb.Page | (vaddr & uint64(tlb.OffsetMask)), nil
		}
		tlb.Clear()
	}

	addr, err := mmu.mapSlow(vaddr, vpn, access)
	if err != nil {
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
		if trap, ok := errors.AsType[*isa.Trap](err); ok && false {
			fmt.Printf("mmu.MapSv39 failed: %v\n", trap)
			if trap.Err != nil {
				fmt.Printf("  caused by %v\n", trap.Err)
			}
		}
		return 0, err
	}

	tlb := &mmu.TLB[vpn&0xfff]

	// Disable this to disable TLB.
	if true {
		tlb.VPN = vpn
		tlb.Page = page
		tlb.Flags = flags | PteV
		tlb.UserMode = mmu.Hart.Mode() == isa.ModeU
	}

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

// MapSv39 does a Sv39 page table lookup for the virtual address.
func (mmu *MMU) MapSv39(root, vaddr uint64, access int) (
	uint64, PTEFlags, int, error) {

	base := root << 12

	for level := 2; level >= 0; level-- {
		idx := index(vaddr, level)
		pteAddr := base + idx*8

		pte := PTE(bo.Uint64(mmu.Mem.RAM[mmu.Mem.Offset(pteAddr):]))

		if !pte.Valid() {
			var err error
			if debugMMU {
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
			return mmu.mapLeaf(pte, pteAddr, vaddr, level, access)
		}

		// Walk to the next level.
		base = pte.PPN() << 12
	}

	return 0, 0, 0, mmu.Hart.Trap(isa.CauseLoadPageFault, vaddr,
		fmt.Errorf("no leaf page found"))
}

// AccessContext provides detailed information about virtual memory access
// faults.
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

func acWithDesc(ac *AccessContext, desc string) *AccessContext {
	if ac != nil {
		ac.Desc = desc
	}
	return ac
}

func (mmu *MMU) mapLeaf(pte PTE, pteAddr, vaddr uint64, level, access int) (
	uint64, PTEFlags, int, error) {

	var ac *AccessContext

	sum := mmu.Hart.Mstatus().SUM()
	mxr := mmu.Hart.Mstatus().MXR()

	if debugMMU {
		ac = &AccessContext{
			Addr:   vaddr,
			PTE:    pte,
			Access: access,
			SUM:    sum,
			MXR:    mxr,
		}
	}

	readable := pte.Readable()
	if mxr && pte.Executable() {
		// MXR overrides read constraints if executable is active
		readable = true
	}
	if pte.Writable() && !readable {
		// W=1, R=0
		return 0, 0, 0, mmu.Hart.Trap(isa.CauseStorePageFault, vaddr, ac)
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
					vaddr, acWithDesc(ac, "U-mode"))
			}
			if access&AccessWrite != 0 {
				return 0, 0, 0, mmu.Hart.Trap(isa.CauseStorePageFault,
					vaddr, acWithDesc(ac, "U-mode"))
			}
			return 0, 0, 0, mmu.Hart.Trap(isa.CauseLoadPageFault,
				vaddr, acWithDesc(ac, "U-mode"))
		}
	} else if mmu.Hart.Mode() == isa.ModeS {
		// Supervisor mode accessing a User Page (PteU == 1)
		if isUserPage {
			// Rule A: Supervisor mode can NEVER execute code from a User page
			if access&AccessExec != 0 {
				return 0, 0, 0, mmu.Hart.Trap(isa.CauseInstPageFault,
					vaddr, acWithDesc(ac, "S-mode exec user page"))
			}
			// Rule B: Supervisor data access is ONLY allowed if
			// sstatus.SUM == 1
			if !sum {
				if access&AccessWrite != 0 {
					return 0, 0, 0, mmu.Hart.Trap(isa.CauseStorePageFault,
						vaddr, acWithDesc(ac, "S-mode user page"))
				}
				return 0, 0, 0, mmu.Hart.Trap(isa.CauseLoadPageFault,
					vaddr, acWithDesc(ac, "S-mode user page"))
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

	var flagsModified bool
	flags := pte.Flags()
	if !flags.Accessed() {
		flags |= PteA
		flagsModified = true
	}
	if access&AccessWrite != 0 && !flags.Dirty() {
		flags |= PteD
		flagsModified = true
	}
	if flagsModified {
		pte.SetFlags(flags)
		bo.PutUint64(mmu.Mem.RAM[mmu.Mem.Offset(pteAddr):], uint64(pte))
	}

	return page, flags, level, nil
}

// Load8 loads a 8-bit value from the virtual address vaddr.
func (mmu *MMU) Load8(vaddr uint64) (uint8, error) {
	paddr, err := mmu.Map(vaddr, AccessRead)
	if err != nil {
		return 0, err
	}
	if paddr < mmu.Mem.RAMBase {
		return mmu.MMIO.Load8(paddr)
	}
	return mmu.Mem.RAM[mmu.Mem.Offset(paddr)], nil
}

// Load16 loads a 16-bit value from the virtual address vaddr.
func (mmu *MMU) Load16(vaddr uint64) (uint16, error) {
	if memory.Avail(vaddr, 2) {
		paddr, err := mmu.Map(vaddr, AccessRead)
		if err != nil {
			return 0, err
		}
		if paddr < mmu.Mem.RAMBase {
			return mmu.MMIO.Load16(paddr)
		}
		if mmu.Mem.Contains(paddr) {
			return bo.Uint16(mmu.Mem.RAM[mmu.Mem.Offset(paddr):]), nil
		}
		if debugMMU {
			err = fmt.Errorf("%v: Load16(%x): addr out of obunds [%x...%x[",
				mmu.Hart.Mode(), paddr,
				mmu.Mem.RAMBase, mmu.Mem.RAMBase+uint64(len(mmu.Mem.RAM)))
			log.Printf("%v", err)
		}
		return 0, mmu.Hart.Trap(isa.CauseLoadPageFault, vaddr, err)
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
				return mmu.MMIO.Load16(paddr)
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

// Load32 loads a 32-bit value from the virtual address vaddr.
func (mmu *MMU) Load32(vaddr uint64) (uint32, error) {
	if memory.Avail(vaddr, 4) {
		paddr, err := mmu.Map(vaddr, AccessRead)
		if err != nil {
			return 0, err
		}
		if paddr < mmu.Mem.RAMBase {
			return mmu.MMIO.Load32(paddr)
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
				return mmu.MMIO.Load32(paddr)
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

// Load64 loads a 64-bit value from the virtual address vaddr.
func (mmu *MMU) Load64(vaddr uint64) (uint64, error) {
	if memory.Avail(vaddr, 8) {
		paddr, err := mmu.Map(vaddr, AccessRead)
		if err != nil {
			return 0, err
		}
		if paddr < mmu.Mem.RAMBase {
			return mmu.MMIO.Load64(paddr)
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
				return mmu.MMIO.Load64(paddr)
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

// Store8 stores a 8-bit value to virtual address vaddr.
func (mmu *MMU) Store8(vaddr uint64, v uint8) error {
	paddr, err := mmu.Map(vaddr, AccessWrite)
	if err != nil {
		return err
	}
	if paddr < mmu.Mem.RAMBase {
		return mmu.MMIO.Store8(paddr, v)
	}
	if mmu.Mem.Contains(paddr) {
		mmu.Mem.RAM[mmu.Mem.Offset(paddr)] = byte(v)
	} else {
		if debugMMU {
			err = fmt.Errorf("%v: Store8(%x, %x): addr out of bounds [%x...%x[",
				mmu.Hart.Mode(), paddr, v,
				mmu.Mem.RAMBase, mmu.Mem.RAMBase+uint64(len(mmu.Mem.RAM)))
			fmt.Printf("%v\r\n", err)
		}
		return mmu.Hart.Trap(isa.CauseStorePageFault, vaddr, err)
	}
	return nil
}

// Store16 stores a 16-bit value to virtual address vaddr.
func (mmu *MMU) Store16(vaddr uint64, v uint16) error {
	if memory.Avail(vaddr, 2) {
		paddr, err := mmu.Map(vaddr, AccessWrite)
		if err != nil {
			return err
		}
		if paddr < mmu.Mem.RAMBase {
			return mmu.MMIO.Store16(paddr, v)
		}
		bo.PutUint16(mmu.Mem.RAM[mmu.Mem.Offset(paddr):], uint16(v))
		return nil

	}
	var buf [2]byte

	bo.PutUint16(buf[:], uint16(v))

	return mmu.CopyToUser(vaddr, buf[:])
}

// Store32 stores a 32-bit value to virtual address vaddr.
func (mmu *MMU) Store32(vaddr uint64, v uint32) error {
	if memory.Avail(vaddr, 4) {
		paddr, err := mmu.Map(vaddr, AccessWrite)
		if err != nil {
			return err
		}
		if paddr < mmu.Mem.RAMBase {
			return mmu.MMIO.Store32(paddr, v)
		}
		bo.PutUint32(mmu.Mem.RAM[mmu.Mem.Offset(paddr):], uint32(v))
		return nil
	}
	var buf [4]byte

	bo.PutUint32(buf[:], uint32(v))

	return mmu.CopyToUser(vaddr, buf[:])
}

// Store64 stores a 64-bit value to virtual address vaddr.
func (mmu *MMU) Store64(vaddr uint64, v uint64) error {
	if memory.Avail(vaddr, 8) {
		paddr, err := mmu.Map(vaddr, AccessWrite)
		if err != nil {
			return err
		}
		if paddr < mmu.Mem.RAMBase {
			return mmu.MMIO.Store64(paddr, v)
		}
		bo.PutUint64(mmu.Mem.RAM[mmu.Mem.Offset(paddr):], v)
		return nil
	}
	var buf [8]byte

	bo.PutUint64(buf[:], v)

	return mmu.CopyToUser(vaddr, buf[:])
}
