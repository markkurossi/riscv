//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package virtio

//lint:file-ignore ST1003 to match the C coding style for constants.

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/markkurossi/riscv/dev"
	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/memory"
)

const (
	BlkSize = 4096
)

const (
	DeviceStatusAcknowledge      = 1
	DeviceStatusDriver           = 2
	DeviceStatusFailed           = 128
	DeviceStatusFeaturesOK       = 8
	DeviceStatusDriverOK         = 4
	DeviceStatusDeviceNeedsReset = 64
)

func statusString(status uint32) string {
	var result []string

	if status&DeviceStatusAcknowledge != 0 {
		result = append(result, "ACKNOWLEDGE")
	}
	if status&DeviceStatusDriver != 0 {
		result = append(result, "DRIVER")
	}
	if status&DeviceStatusFailed != 0 {
		result = append(result, "FAILED")
	}
	if status&DeviceStatusFeaturesOK != 0 {
		result = append(result, "FEATURES_OK")
	}
	if status&DeviceStatusDriverOK != 0 {
		result = append(result, "DRIVER_OK")
	}
	if status&DeviceStatusDeviceNeedsReset != 0 {
		result = append(result, "DEVICE_NEEDS_RESET")
	}

	return strings.Join(result, ",")
}

type Blk struct {
	Hart     isa.Hart
	Start    uint64
	End      uint64
	Plic     *dev.PLIC
	IRQ      uint32
	Mem      *memory.Memory
	File     *os.File
	fileInfo os.FileInfo

	deviceFeaturesSel uint32
	driverFeaturesSel uint32
	driverFeatures    [2]uint32
	status            uint32
	interruptStatus   uint32

	queueSel uint32
	queues   [1]VirtQueue
}

func NewBlk(hart isa.Hart, start uint64, plic *dev.PLIC, irq uint32,
	mem *memory.Memory, file *os.File) *Blk {

	blk := &Blk{
		Hart:  hart,
		Start: start,
		End:   start + BlkSize,
		Plic:  plic,
		IRQ:   irq,
		Mem:   mem,
		File:  file,
	}
	for idx := range blk.queues {
		blk.queues[idx].Blk = blk
	}

	return blk
}

type VirtQueue struct {
	Blk          *Blk
	Num          uint32 // Set by guest (num of descs allocated, <= NumMax)
	Ready        uint32 // Guest writes 1 to activate
	DescPhys     uint64 // 64-bit Guest Physical Address of Descriptor Table
	AvailPhys    uint64 // 64-bit Guest Physical Address of Available Ring
	UsedPhys     uint64 // 64-bit Guest Physical Address of Used Ring
	lastAvailIdx uint16
}

func (vq *VirtQueue) executeDescriptorChain(idx uint16) error {
	vq.Blk.logf("chain: idx=%v", idx)

	hdr, err := vq.loadDesc(idx)
	if err != nil {
		return err
	}
	if hdr.Len != 16 || hdr.Flags&VIRTQ_DESC_F_NEXT == 0 {
		return fmt.Errorf("invalid blk header: %v", hdr)
	}
	t, err := vq.Blk.readGuestUint32(hdr.Addr)
	if err != nil {
		return err
	}
	sector, err := vq.Blk.readGuestUint64(hdr.Addr + 8)
	if err != nil {
		return err
	}

	data, err := vq.loadDesc(hdr.Next)
	if err != nil {
		return err
	}
	if data.Flags&VIRTQ_DESC_F_NEXT == 0 {
		return fmt.Errorf("invalid blk data: %v", data)
	}

	status, err := vq.loadDesc(data.Next)
	if err != nil {
		return err
	}
	if status.Len != 1 {
		return fmt.Errorf("invalid blk status: %v", status)
	}

	fileOffset := int64(sector) * 512
	addr := data.Addr

	// Handle request type.

	var opStatus uint8 = VIRTIO_BLK_S_OK

	switch t {
	case VIRTIO_BLK_T_IN:
		buf, err := vq.Blk.guestData(addr, uint64(data.Len))
		if err != nil {
			// XXX set status to err.
			vq.Blk.logf("guestData(%v,%v) failed: %v",
				addr, data.Len, err)
			return err
		}
		n, err := vq.Blk.File.ReadAt(buf, fileOffset)
		if err != nil {
			vq.Blk.logf("failed to read from host file at offset %d: %v",
				fileOffset, err)
			opStatus = VIRTIO_BLK_S_IOERR
		} else {
			vq.Blk.logf("read %d/%d bytes into guest RAM addr %x (offset %x)",
				n, data.Len, addr, vq.Blk.Mem.Offset(addr))
		}

	case VIRTIO_BLK_T_OUT:
		buf, err := vq.Blk.guestData(addr, uint64(data.Len))
		if err != nil {
			// XXX set status to err.
			vq.Blk.logf("guestData(%v,%v) failed: %v",
				addr, data.Len, err)
			return err
		}
		n, err := vq.Blk.File.WriteAt(buf, fileOffset)
		if err != nil {
			vq.Blk.logf("failed to write to host file at offset %d: %v",
				fileOffset, err)
			opStatus = VIRTIO_BLK_S_IOERR
		} else {
			vq.Blk.logf("wrote %d/%d bytes from guest RAM addr %x (offset %x)",
				n, data.Len, addr, vq.Blk.Mem.Offset(addr))
		}

	default:
		vq.Blk.logf("type %v not supported", t)
		opStatus = VIRTIO_BLK_S_UNSUPP
	}

	err = vq.Blk.writeGuestUint8(status.Addr, opStatus)
	if err != nil {
		return err
	}
	vq.Blk.logf("req header: %v\n", hdr)
	vq.Blk.logf(" - type   : %v\n", blkTypeString(t))
	vq.Blk.logf(" - sector : %v\n", sector)
	vq.Blk.logf("req data  : %v\n", data)
	vq.Blk.logf("req status: %v\n", status)

	// Update used ring.
	err = vq.updateUsedRing(idx, data.Len)
	if err != nil {
		// XXX update status[0].
		return err
	}

	return nil
}

func (vq *VirtQueue) updateUsedRing(idx uint16, bytesTransferred uint32) error {
	usedIdxAddr := vq.UsedPhys + 2
	usedIdx, err := vq.Blk.readGuestUint16(usedIdxAddr)
	if err != nil {
		return err
	}

	vq.Blk.logf("updateUsedRing: usedIdx=%v, idx=%v, transferred=%v\n",
		usedIdx, idx, bytesTransferred)
	vq.Blk.Hart.SetTrace(true)

	elemAddr := vq.UsedPhys + 4 + (uint64(uint32(usedIdx)%vq.Num) * 8)

	// ID of the descriptor chain head
	vq.Blk.writeGuestUint32(elemAddr, uint32(idx))
	vq.Blk.writeGuestUint32(elemAddr+4, bytesTransferred)

	// 2. Increment the Used Ring total tracker index
	vq.Blk.writeGuestUint16(usedIdxAddr, usedIdx+1)

	// 3. Set the Interrupt Status Register to notify the guest
	vq.Blk.interruptStatus |= 0x1 // Virtqueue Interrupt 0x1

	// 4. Assert your PLIC line!
	vq.Blk.Plic.SetInterruptRequest(vq.Blk.IRQ, true)

	return nil
}

func (vq *VirtQueue) loadDesc(idx uint16) (*VirtioDesc, error) {
	desc := vq.DescPhys + uint64(idx*16)

	addr, err := vq.Blk.readGuestUint64(desc)
	if err != nil {
		return nil, err
	}
	l, err := vq.Blk.readGuestUint32(desc + 8)
	if err != nil {
		return nil, err
	}
	flags, err := vq.Blk.readGuestUint16(desc + 12)
	if err != nil {
		return nil, err
	}
	next, err := vq.Blk.readGuestUint16(desc + 14)
	if err != nil {
		return nil, err
	}

	return &VirtioDesc{
		Addr:  addr,
		Len:   l,
		Flags: flags,
		Next:  next,
	}, nil
}

const (
	VIRTQ_DESC_F_NEXT     = 1 // Next field valid.
	VIRTQ_DESC_F_WRITE    = 2 // Device write-only (otherwise read-only)
	VIRTQ_DESC_F_INDIRECT = 4 // List of buffer descriptors.
)

type VirtioDesc struct {
	Addr  uint64 // Guest Physical Address
	Len   uint32 // Length of the buffer
	Flags uint16 // VIRTIO_DESC_F_NEXT (1), VIRTIO_DESC_F_WRITE (2)
	Next  uint16 // If flag NEXT is set, the index of the next descriptor
}

func (desc *VirtioDesc) String() string {
	return fmt.Sprintf("Buf=%v@%x,Flags=%x,Next=%x",
		desc.Len, desc.Addr, desc.Flags, desc.Next)
}

const (
	VIRTIO_BLK_T_IN           = 0
	VIRTIO_BLK_T_OUT          = 1
	VIRTIO_BLK_T_FLUSH        = 4
	VIRTIO_BLK_T_DISCARD      = 11
	VIRTIO_BLK_T_WRITE_ZEROES = 13
)

var blkTypes = map[uint32]string{
	VIRTIO_BLK_T_IN:           "read",
	VIRTIO_BLK_T_OUT:          "write",
	VIRTIO_BLK_T_FLUSH:        "flush",
	VIRTIO_BLK_T_DISCARD:      "discard",
	VIRTIO_BLK_T_WRITE_ZEROES: "write-zeroes",
}

func blkTypeString(t uint32) string {
	name, ok := blkTypes[t]
	if ok {
		return name
	}
	return fmt.Sprintf("{type %d}", t)
}

const (
	VIRTIO_BLK_S_OK     = 0
	VIRTIO_BLK_S_IOERR  = 1
	VIRTIO_BLK_S_UNSUPP = 2
)

func (vio *Blk) logf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Print("virtio-blk: " + msg)
}

func (vio *Blk) Halt() error {
	return nil
}

func (vio *Blk) Contains(paddr uint64) bool {
	return paddr >= vio.Start && paddr < vio.End
}

func (vio *Blk) Load8(paddr uint64) (uint8, error) {
	vio.logf("Load8(0x%03x)", paddr-vio.Start)
	return 0, nil
}

func (vio *Blk) Load16(paddr uint64) (uint16, error) {
	vio.logf("Load16(0x%03x)", paddr-vio.Start)
	return 0, nil
}

func (vio *Blk) Load32(paddr uint64) (uint32, error) {
	offset := paddr - vio.Start

	vio.logf("Load32(%v)", mmioReg(offset))

	switch offset {
	case 0x000:
		return Magic, nil
	case 0x004:
		return Version, nil
	case 0x008:
		return DeviceID, nil
	case 0x00c:
		return VendorID, nil
	case 0x010: // DeviceFeatures
		switch vio.deviceFeaturesSel {
		case 0:
			// Bits 0-31: Block-specific features (e.g., RO, Segments, etc.)
			// Return 0 for now if you want a basic default disk
			return 0, nil
		case 1:
			// Bits 32-63: Generic VirtIO features
			// Bit 32 is VIRTIO_F_VERSION_1 (1 << 0 of this upper page)
			return 1 << 0, nil
		}

	case 0x034: // QueueNumMax
		if vio.queueSel < uint32(len(vio.queues)) {
			return 256, nil
		}
		return 0, nil

	case 0x044: // QueueReady
		if vio.queueSel < uint32(len(vio.queues)) {
			return vio.queues[vio.queueSel].Ready, nil
		}
		return 0, nil

	case 0x060: // InterruptStatus
		return vio.interruptStatus, nil

	case 0x070:
		vio.logf("Load32(%v) => %v[0x%x]\n", mmioReg(offset),
			statusString(vio.status), vio.status)
		return vio.status, nil

	case 0x100: // Disk size in sectors, low
		return uint32(vio.size()), nil

	case 0x104: // Disk size in sectors, high
		return uint32(vio.size() >> 32), nil
	}

	return 0, nil
}

func (vio *Blk) Load64(paddr uint64) (uint64, error) {
	vio.logf("Load64(0x%03x)", paddr-vio.Start)
	return 0, nil
}

func (vio *Blk) Store8(paddr, v uint64) error {
	vio.logf("Store8(0x%03x, 0x%02x)", paddr-vio.Start, v)
	return nil
}

func (vio *Blk) Store16(paddr, v uint64) error {
	vio.logf("Store16(0x%03x, 0x%04x)", paddr-vio.Start, v)
	return nil
}

func (vio *Blk) Store32(paddr, v uint64) error {
	offset := paddr - vio.Start

	vio.logf("Store32(%v, 0x%08x)", mmioReg(offset), v)

	switch offset {
	case 0x014: // DeviceFeaturesSel
		vio.deviceFeaturesSel = uint32(v)

	case 0x024: // DriverFeaturesSel
		vio.driverFeaturesSel = uint32(v)

	case 0x020: // DriverFeatures
		if vio.driverFeaturesSel < 2 {
			vio.driverFeatures[vio.driverFeaturesSel] = uint32(v)
		}

	case 0x030: // QueueSel
		vio.queueSel = uint32(v)

	case 0x038: // QueueNum (Guest setting chosen queue size)
		if vio.queueSel < uint32(len(vio.queues)) {
			vio.queues[vio.queueSel].Num = uint32(v)
		}

	case 0x044: // QueueReady
		if vio.queueSel < uint32(len(vio.queues)) {
			vio.queues[vio.queueSel].Ready = uint32(v)
		}

	case 0x050: // QueueNotify
		vio.processQueue(uint32(v))

	case 0x064: // InterruptACK The guest writes a bitmask of the bits
		vio.logf("InterruptACK status=%x", vio.interruptStatus)

		// it has acknowledged and wants cleared
		vio.interruptStatus &^= uint32(v)
		vio.logf("interruptStatus: %x\n", vio.interruptStatus)

		// If the guest has cleared all active interrupts, we can
		// de-assert the PLIC line
		if vio.interruptStatus == 0 {
			// Lower the interrupt line
			vio.logf("clearing PLIC interrupt %v", vio.IRQ)
			vio.Plic.SetInterruptRequest(vio.IRQ, false)
		}

		// Buffer Base Address Registers (rv64 writes lower then upper halves)

	case 0x080: // QueueDescLow
		vio.queues[0].DescPhys =
			(vio.queues[0].DescPhys & 0xffffffff00000000) | (v & 0xffffffff)

	case 0x084: // QueueDescHigh
		vio.queues[0].DescPhys =
			(vio.queues[0].DescPhys & 0x00000000ffffffff) |
				(uint64(v&0xffffffff) << 32)

	case 0x090: // QueueDriverLow (Available Ring)
		vio.queues[0].AvailPhys =
			(vio.queues[0].AvailPhys & 0xffffffff00000000) | (v & 0xffffffff)

	case 0x094: // QueueDriverHigh
		vio.queues[0].AvailPhys =
			(vio.queues[0].AvailPhys & 0x00000000ffffffff) |
				(uint64(v&0xffffffff) << 32)

	case 0x0a0: // QueueDeviceLow (Used Ring)
		vio.queues[0].UsedPhys =
			(vio.queues[0].UsedPhys & 0xffffffff00000000) | (v & 0xffffffff)

	case 0x0a4: // QueueDeviceHigh
		vio.queues[0].UsedPhys =
			(vio.queues[0].UsedPhys & 0x00000000ffffffff) |
				(uint64(v&0xffffffff) << 32)

	case 0x070: // Status
		if v == 0 {
			// Guest requested a device reset
			vio.status = 0
			vio.driverFeatures[0] = 0
			vio.driverFeatures[1] = 0
			return nil
		}

		// Standard status update protocol sequence
		vio.status |= uint32(v)

		if v&8 != 0 { // VIRTIO_CONFIG_S_FEATURES_OK (8)
			// Validate that the driver acknowledged
			// VIRTIO_F_VERSION_1 (bit 32 -> page 1, bit 0)
			if (vio.driverFeatures[1] & 1) == 0 {
				// If driver rejected version 1, clear the FEATURES_OK
				// bit to signal failure
				vio.status &= ^uint32(8)
			}
		}
	}
	return nil
}

func (vio *Blk) Store64(paddr, v uint64) error {
	vio.logf("Store64(0x%03x, 0x%016x)", paddr-vio.Start, v)
	return nil
}

func (vio *Blk) size() uint64 {
	var err error

	if vio.fileInfo == nil {
		vio.fileInfo, err = vio.File.Stat()
		if err != nil {
			vio.logf("failed to stat image: %v", err)
			return 0
		}
	}
	return uint64((vio.fileInfo.Size() + 511) / 512)
}

func (vio *Blk) guestData(addr, l uint64) ([]byte, error) {
	if !vio.Mem.Contains(addr) {
		return nil, fmt.Errorf("invalid addr %x", addr)
	}
	ofs := vio.Mem.Offset(addr)
	return vio.Mem.RAM[ofs : ofs+l], nil
}

func (vio *Blk) readGuestUint8(addr uint64) (uint8, error) {
	if !vio.Mem.Contains(addr) {
		return 0, fmt.Errorf("invalid addr %x", addr)
	}
	return vio.Mem.RAM[vio.Mem.Offset(addr)], nil
}

func (vio *Blk) readGuestUint16(addr uint64) (uint16, error) {
	if !vio.Mem.Contains(addr) {
		return 0, fmt.Errorf("invalid addr %x", addr)
	}
	return vio.Mem.BO.Uint16(vio.Mem.RAM[vio.Mem.Offset(addr):]), nil
}

func (vio *Blk) readGuestUint32(addr uint64) (uint32, error) {
	if !vio.Mem.Contains(addr) {
		return 0, fmt.Errorf("invalid addr %x", addr)
	}
	return vio.Mem.BO.Uint32(vio.Mem.RAM[vio.Mem.Offset(addr):]), nil
}

func (vio *Blk) readGuestUint64(addr uint64) (uint64, error) {
	if !vio.Mem.Contains(addr) {
		return 0, fmt.Errorf("invalid addr %x", addr)
	}
	return vio.Mem.BO.Uint64(vio.Mem.RAM[vio.Mem.Offset(addr):]), nil
}

func (vio *Blk) writeGuestUint8(addr uint64, v uint8) error {
	if !vio.Mem.Contains(addr) {
		return fmt.Errorf("invalid addr %x", addr)
	}
	vio.Mem.RAM[vio.Mem.Offset(addr)] = v

	return nil
}

func (vio *Blk) writeGuestUint16(addr uint64, v uint16) error {
	if !vio.Mem.Contains(addr) {
		return fmt.Errorf("invalid addr %x", addr)
	}
	vio.Mem.BO.PutUint16(vio.Mem.RAM[vio.Mem.Offset(addr):], v)

	return nil
}

func (vio *Blk) writeGuestUint32(addr uint64, v uint32) error {
	if !vio.Mem.Contains(addr) {
		return fmt.Errorf("invalid addr %x", addr)
	}
	vio.Mem.BO.PutUint32(vio.Mem.RAM[vio.Mem.Offset(addr):], v)

	return nil
}

func (vio *Blk) processQueue(idx uint32) {
	vio.logf("QueueNotify(%v)", idx)

	if idx >= uint32(len(vio.queues)) {
		vio.logf("invalid queue index  %v", idx)
		return
	}

	// 1. Read the driver's current Available Ring Index from guest memory
	//
	//   Avail Ring layout: flags (2 bytes), idx (2 bytes), ring[...]
	//   (array of uint16)
	paddr := vio.queues[idx].AvailPhys + 2
	for {
		availIdx, err := vio.readGuestUint16(paddr)
		if err != nil {
			vio.logf("guest memory access: %v", err)
			return
		}
		vio.logf("queue: idx: %v/%v", vio.queues[idx].lastAvailIdx, availIdx)

		if vio.queues[idx].lastAvailIdx == availIdx {
			// Queue drained.
			break
		}
		ringOffset := uint64(uint32(vio.queues[idx].lastAvailIdx) %
			vio.queues[idx].Num)
		paddr := vio.queues[idx].AvailPhys + 4 + (ringOffset * 2)
		descHeadIdx, err := vio.readGuestUint16(paddr)
		if err != nil {
			vio.logf("guest memory access: %v", err)
			return
		}
		vio.logf("queue: idx=%v, ringOffset=%v, paddr=%x, headIdx=%v",
			vio.queues[idx].lastAvailIdx, ringOffset, paddr, descHeadIdx)

		err = vio.queues[idx].executeDescriptorChain(descHeadIdx)
		if err != nil {
			vio.logf("execute descriptor chain: %v", err)
			return
		}

		vio.queues[idx].lastAvailIdx++
	}
}
