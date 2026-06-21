//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

// Package virtio implements the Virtual I/O Device (VIRTIO) Version 1.1.
package virtio

import (
	"fmt"
	"strings"
	"sync"

	"github.com/markkurossi/riscv/dev"
	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/logger"
	"github.com/markkurossi/riscv/memory"
	"github.com/markkurossi/riscv/mmu"
)

const (
	Magic    = 0x74726976 // "virt"
	Version  = 0x2
	VendorID = 0x476f4d55 // "GoMU"

	FeatureAnyLayout = 1 << 28
	FeatureVersion1  = 1 << 31

	queueNumMax = 512
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

var (
	_ mmu.MMIO = &MMIO{}
)

type MMIO struct {
	logger.Logger
	M        sync.Mutex
	C        *sync.Cond
	Name     string
	DeviceID uint32
	Features uint32
	Hart     isa.Hart
	Start    uint64
	End      uint64
	Plic     *dev.PLIC
	IRQ      uint32
	Mem      *memory.Memory
	Handler  Handler

	deviceFeaturesSel uint32
	driverFeaturesSel uint32
	driverFeatures    [2]uint32
	status            uint32
	interruptStatus   uint32

	queueSel uint32
	queues   []Queue
}

type Handler interface {
	Reset() error
	ExecuteDescriptorChain(vq *Queue, idx uint16) (uint32, error)
}

func (vio *MMIO) Device() *MMIO {
	return vio
}

func (vio *MMIO) Init(numQueues int) {
	vio.C = sync.NewCond(&vio.M)

	for idx := range numQueues {
		vio.queues = append(vio.queues, Queue{
			MMIO:  vio,
			Index: idx,
		})
	}
}

// Halt implements mmu.ROM.Halt.
func (vio *MMIO) Halt() error {
	return nil
}

func (vio *MMIO) Contains(paddr uint64) bool {
	return vio.Start <= paddr && paddr < vio.End
}

func (vio *MMIO) Load8(paddr uint64) (uint8, error) {
	return 0, fmt.Errorf("MMIO.Load8(%x)", paddr-vio.Start)
}

func (vio *MMIO) Load16(paddr uint64) (uint16, error) {
	return 0, fmt.Errorf("MMIO.Load16(%x)", paddr-vio.Start)
}

func (vio *MMIO) Load32(paddr uint64) (uint32, error) {
	offset := paddr - vio.Start

	vio.Debugf("Load32(%v)", mmioReg(offset))

	switch offset {
	case 0x000:
		return Magic, nil
	case 0x004:
		return Version, nil
	case 0x008:
		return vio.DeviceID, nil
	case 0x00c:
		return VendorID, nil
	case 0x010: // DeviceFeatures
		switch vio.deviceFeaturesSel {
		case 0:
			// Bits 0-31: device specific features.
			return vio.Features, nil
		case 1:
			// Bits 32-63: Generic VirtIO features.
			// Bit 32 is VIRTIO_F_VERSION_1 (1 << 0 of this upper page)
			return 1 << 0, nil
		}

	case 0x034: // QueueNumMax
		if vio.queueSel < uint32(len(vio.queues)) {
			return queueNumMax, nil
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
		vio.Debugf("Load32(%v) => %v[0x%x]\n", mmioReg(offset),
			statusString(vio.status), vio.status)
		return vio.status, nil
	}
	return 0, nil
}

func (vio *MMIO) Load64(paddr uint64) (uint64, error) {
	return 0, fmt.Errorf("MMIO.Load64(%x)", paddr-vio.Start)
}

func (vio *MMIO) Store8(paddr uint64, v uint8) error {
	return fmt.Errorf("MMIO.Store8(%x, 0x%02x)", paddr-vio.Start, v)
}

func (vio *MMIO) Store16(paddr uint64, v uint16) error {
	return fmt.Errorf("MMIO.Store16(%x, 0x%04x)", paddr-vio.Start, v)
}

func (vio *MMIO) Store32(paddr uint64, v uint32) error {
	offset := paddr - vio.Start

	vio.Debugf("Store32(%v, 0x%08x)", mmioReg(offset), v)

	switch offset {
	case 0x014: // DeviceFeaturesSel
		vio.deviceFeaturesSel = v

	case 0x024: // DriverFeaturesSel
		vio.driverFeaturesSel = v

	case 0x020: // DriverFeatures
		if vio.driverFeaturesSel < 2 {
			vio.driverFeatures[vio.driverFeaturesSel] = v
		}

	case 0x030: // QueueSel
		vio.queueSel = v

	case 0x038: // QueueNum (Guest setting chosen queue size)
		if vio.queueSel < uint32(len(vio.queues)) {
			vio.queues[vio.queueSel].Num = v
		}

	case 0x044: // QueueReady
		if vio.queueSel < uint32(len(vio.queues)) {
			vio.queues[vio.queueSel].Ready = v
		}

	case 0x050: // QueueNotify
		vio.ProcessQueue(v)

	case 0x064: // InterruptACK The guest writes a bitmask of the bits
		vio.Debugf("InterruptACK status=%x", vio.interruptStatus)

		// it has acknowledged and wants cleared
		vio.interruptStatus &^= v
		vio.Debugf("interruptStatus: %x\n", vio.interruptStatus)

		// If the guest has cleared all active interrupts, we can
		// de-assert the PLIC line
		if vio.interruptStatus == 0 {
			// Lower the interrupt line
			vio.Debugf("clearing PLIC interrupt %v", vio.IRQ)
			vio.Plic.SetInterruptRequest(vio.IRQ, false)
		} else {
			vio.Debugf("setting PLIC interrupt %v", vio.IRQ)
			vio.Plic.SetInterruptRequest(vio.IRQ, true)
		}

	case 0x070: // Status
		if v == 0 {
			// Guest requested a device reset
			vio.deviceFeaturesSel = 0
			vio.driverFeaturesSel = 0
			vio.driverFeatures[0] = 0
			vio.driverFeatures[1] = 0
			vio.status = 0
			vio.queueSel = 0

			for idx := range vio.queues {
				vio.queues[idx].Num = 0
				vio.queues[idx].Ready = 0
				vio.queues[idx].DescPhys = 0
				vio.queues[idx].AvailPhys = 0
				vio.queues[idx].UsedPhys = 0
				vio.queues[idx].lastAvailIdx = 0
			}

			return vio.Handler.Reset()
		}

		// Standard status update protocol sequence
		vio.status |= v

		if v&8 != 0 { // VIRTIO_CONFIG_S_FEATURES_OK (8)
			// Validate that the driver acknowledged
			// VIRTIO_F_VERSION_1 (bit 32 -> page 1, bit 0)
			if (vio.driverFeatures[1] & 1) == 0 {
				// If driver rejected version 1, clear the FEATURES_OK
				// bit to signal failure
				vio.status &= ^uint32(8)
			}
		}

		// Buffer Base Address Registers (rv64 writes lower then upper halves)

	case 0x080: // QueueDescLow
		if vio.queueSel < uint32(len(vio.queues)) {
			vio.queues[vio.queueSel].DescPhys =
				(vio.queues[vio.queueSel].DescPhys & 0xffffffff00000000) |
					uint64(v)
		}

	case 0x084: // QueueDescHigh
		if vio.queueSel < uint32(len(vio.queues)) {
			vio.queues[vio.queueSel].DescPhys =
				(vio.queues[vio.queueSel].DescPhys & 0x00000000ffffffff) |
					(uint64(v) << 32)
		}

	case 0x090: // QueueDriverLow (Available Ring)
		if vio.queueSel < uint32(len(vio.queues)) {
			vio.queues[vio.queueSel].AvailPhys =
				(vio.queues[vio.queueSel].AvailPhys & 0xffffffff00000000) |
					uint64(v)
		}

	case 0x094: // QueueDriverHigh
		if vio.queueSel < uint32(len(vio.queues)) {
			vio.queues[vio.queueSel].AvailPhys =
				(vio.queues[vio.queueSel].AvailPhys & 0x00000000ffffffff) |
					(uint64(v) << 32)
		}

	case 0x0a0: // QueueDeviceLow (Used Ring)
		if vio.queueSel < uint32(len(vio.queues)) {
			vio.queues[vio.queueSel].UsedPhys =
				(vio.queues[vio.queueSel].UsedPhys & 0xffffffff00000000) |
					uint64(v)
		}

	case 0x0a4: // QueueDeviceHigh
		if vio.queueSel < uint32(len(vio.queues)) {
			vio.queues[vio.queueSel].UsedPhys =
				(vio.queues[vio.queueSel].UsedPhys & 0x00000000ffffffff) |
					(uint64(v) << 32)
		}
	}
	return nil
}

func (vio *MMIO) Store64(paddr uint64, v uint64) error {
	return fmt.Errorf("MMIO.Store64(%x, 0x%016x)", paddr-vio.Start, v)
}

func (vio *MMIO) guestData(addr, l uint64) ([]byte, error) {
	if !vio.Mem.Contains(addr) {
		return nil, fmt.Errorf("invalid addr %x", addr)
	}
	ofs := vio.Mem.Offset(addr)
	return vio.Mem.RAM[ofs : ofs+l], nil
}

func (vio *MMIO) readGuestUint8(addr uint64) (uint8, error) {
	if !vio.Mem.Contains(addr) {
		return 0, fmt.Errorf("invalid addr %x", addr)
	}
	return vio.Mem.RAM[vio.Mem.Offset(addr)], nil
}

func (vio *MMIO) readGuestUint16(addr uint64) (uint16, error) {
	if !vio.Mem.Contains(addr) {
		return 0, fmt.Errorf("invalid addr %x", addr)
	}
	return vio.Mem.BO.Uint16(vio.Mem.RAM[vio.Mem.Offset(addr):]), nil
}

func (vio *MMIO) readGuestUint32(addr uint64) (uint32, error) {
	if !vio.Mem.Contains(addr) {
		return 0, fmt.Errorf("invalid addr %x", addr)
	}
	return vio.Mem.BO.Uint32(vio.Mem.RAM[vio.Mem.Offset(addr):]), nil
}

func (vio *MMIO) readGuestUint64(addr uint64) (uint64, error) {
	if !vio.Mem.Contains(addr) {
		return 0, fmt.Errorf("invalid addr %x", addr)
	}
	return vio.Mem.BO.Uint64(vio.Mem.RAM[vio.Mem.Offset(addr):]), nil
}

func (vio *MMIO) writeGuestUint8(addr uint64, v uint8) error {
	if !vio.Mem.Contains(addr) {
		return fmt.Errorf("invalid addr %x", addr)
	}
	vio.Mem.RAM[vio.Mem.Offset(addr)] = v

	return nil
}

func (vio *MMIO) writeGuestUint16(addr uint64, v uint16) error {
	if !vio.Mem.Contains(addr) {
		return fmt.Errorf("invalid addr %x", addr)
	}
	vio.Mem.BO.PutUint16(vio.Mem.RAM[vio.Mem.Offset(addr):], v)

	return nil
}

func (vio *MMIO) writeGuestUint32(addr uint64, v uint32) error {
	if !vio.Mem.Contains(addr) {
		return fmt.Errorf("invalid addr %x", addr)
	}
	vio.Mem.BO.PutUint32(vio.Mem.RAM[vio.Mem.Offset(addr):], v)

	return nil
}

func (vio *MMIO) ProcessQueue(idx uint32) {
	vio.Debugf("QueueNotify(%v)", idx)

	if idx >= uint32(len(vio.queues)) {
		vio.Errorf("invalid queue index  %v", idx)
		return
	}

	vq := &vio.queues[idx]
	availIdxAddr := vq.AvailPhys + 2

	// Read the boundary target limit from the driver
	availIdx, err := vio.readGuestUint16(availIdxAddr)
	if err != nil {
		vio.Errorf("guest memory access: %v", err)
		return
	}

	// Read the baseline state of the used index ring counter ONCE
	// before looping
	usedIdxAddr := vq.UsedPhys + 2
	usedIdx, err := vio.readGuestUint16(usedIdxAddr)
	if err != nil {
		vio.Errorf("guest memory access: %v", err)
		return
	}

	var processedAny bool

	// Process all pending descriptors currently batched in this pass
	for vq.lastAvailIdx != availIdx {
		ringOffset := uint64(uint32(vq.lastAvailIdx) % vq.Num)
		ringElementPhys := vq.AvailPhys + 4 + (ringOffset * 2)
		descHeadIdx, err := vio.readGuestUint16(ringElementPhys)
		if err != nil {
			vio.Errorf("guest memory access: %v", err)
			return
		}

		// Execute the chain (Modify executeDescriptorChain to NOT
		// call updateUsedRing inside it!)
		transferred, err := vio.Handler.ExecuteDescriptorChain(vq, descHeadIdx)
		if err != nil {
			vio.Errorf("execute descriptor chain: %v", err)
			return
		}

		// Write back this completed entry into the Used Ring array
		elemAddr := vq.UsedPhys + 4 + (uint64(uint32(usedIdx)%vq.Num) * 8)
		vio.writeGuestUint32(elemAddr, uint32(descHeadIdx))
		vio.writeGuestUint32(elemAddr+4, transferred)

		usedIdx++
		vq.lastAvailIdx++
		processedAny = true
	}

	if processedAny {
		// Flush the collective index batch change back to guest RAM ONCE
		vio.writeGuestUint16(usedIdxAddr, usedIdx)

		// Read the guest's Available Ring flags (offset 0 of
		// AvailPhys) to see if they have suppressed interrupts
		availFlags, err := vio.readGuestUint16(vq.AvailPhys)
		if err != nil {
			vio.Errorf("guest memory access: %v", err)
			return
		}

		// Bit 0 of avail ring flags is VIRTQ_AVAIL_F_NO_INTERRUPT (1)
		if (availFlags & 1) == 0 {
			// Only inject the interrupt if the driver hasn't
			// explicitly suppressed it!
			vio.interruptStatus |= 0x1
			vio.Plic.SetInterruptRequest(vio.IRQ, true)
		} else {
			vio.Debugf("interrupt suppressed by guest driver")
		}
	}
}

var mmioRegs = map[uint64]string{
	0x000: "Magic",
	0x004: "Version",
	0x008: "DeviceID",
	0x00c: "VendorID",
	0x010: "DeviceFeatures",
	0x014: "DeviceFeaturesSel",
	0x020: "DriverFeatures",
	0x024: "DriverFeaturesSel",
	0x030: "QueueSel",
	0x034: "QueueNumMax",
	0x038: "QueueNum",
	0x044: "QueueReady",
	0x050: "QueueNotify",
	0x060: "InterruptStatus",
	0x064: "InterruptACK",
	0x070: "Status",
	0x080: "QueueDescLow",
	0x084: "QueueDescHigh",
	0x090: "QueueDriverLow",
	0x094: "QueueDriverHigh",
	0x0a0: "QueueDeviceLow",
	0x0a4: "QueueDeviceHigh",
	0x0fc: "ConfigGeneration",
}

func mmioReg(ofs uint64) string {
	name, ok := mmioRegs[ofs]
	if ok {
		return fmt.Sprintf("%v[0x%03x]", name, ofs)
	}
	return fmt.Sprintf("0x%03x", ofs)
}
