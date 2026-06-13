//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package virtio

import (
	"fmt"
)

type VirtQueueNew struct {
	MMIO         *MMIO
	Num          uint32 // Set by guest (num of descs allocated, <= NumMax)
	Ready        uint32 // Guest writes 1 to activate
	DescPhys     uint64 // 64-bit Guest Physical Address of Descriptor Table
	AvailPhys    uint64 // 64-bit Guest Physical Address of Available Ring
	UsedPhys     uint64 // 64-bit Guest Physical Address of Used Ring
	lastAvailIdx uint16
}

func (vq *VirtQueueNew) loadDesc(idx uint16) (*VirtioDescNew, error) {
	desc := vq.DescPhys + uint64(idx*16)

	addr, err := vq.MMIO.readGuestUint64(desc)
	if err != nil {
		return nil, err
	}
	l, err := vq.MMIO.readGuestUint32(desc + 8)
	if err != nil {
		return nil, err
	}
	flags, err := vq.MMIO.readGuestUint16(desc + 12)
	if err != nil {
		return nil, err
	}
	next, err := vq.MMIO.readGuestUint16(desc + 14)
	if err != nil {
		return nil, err
	}

	return &VirtioDescNew{
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

type VirtioDescNew struct {
	Addr  uint64 // Guest Physical Address
	Len   uint32 // Length of the buffer
	Flags uint16 // VIRTIO_DESC_F_NEXT (1), VIRTIO_DESC_F_WRITE (2)
	Next  uint16 // If flag NEXT is set, the index of the next descriptor
}

func (desc *VirtioDescNew) String() string {
	return fmt.Sprintf("Buf=%v@%x,Flags=%x,Next=%x",
		desc.Len, desc.Addr, desc.Flags, desc.Next)
}
