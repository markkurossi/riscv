//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package virtio

import (
	"fmt"

	"github.com/markkurossi/riscv/dev"
	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/memory"
)

const (
	NetDeviceID = 0x1
	NetSize     = 4096
)

// Feature bits.
const (
	// Device handles packets with partial checksum offload.
	VIRTIO_NET_F_CSUM = 0

	// Driver handles packets with partial checksum.
	VIRTIO_NET_F_GUEST_CSUM = 1

	// Control channel offloads reconfiguration support.
	VIRTIO_NET_F_CTRL_GUEST_OFFLOADS = 2

	// Device maximum MTU reporting is supported. If offered by the
	// device, device advises driver about the value of its maximum
	// MTU. If negotiated, the driver uses mtu as the maximum MTU
	// value.
	VIRTIO_NET_F_MTU = 3

	// Device has given MAC address.
	VIRTIO_NET_F_MAC = 5

	// Driver can receive TSOv4.
	VIRTIO_NET_F_GUEST_TSO4 = 7

	// Driver can receive TSOv6.
	VIRTIO_NET_F_GUEST_TSO6 = 8

	// Driver can receive TSO with ECN.
	VIRTIO_NET_F_GUEST_ECN = 9

	// Driver can receive UFO.
	VIRTIO_NET_F_GUEST_UFO = 10

	// Device can receive TSOv4.
	VIRTIO_NET_F_HOST_TSO4 = 11

	// Device can receive TSOv6.
	VIRTIO_NET_F_HOST_TSO6 = 12

	// Device can receive TSO with ECN.
	VIRTIO_NET_F_HOST_ECN = 13

	// Device can receive UFO.
	VIRTIO_NET_F_HOST_UFO = 14

	// Driver can merge receive buffers.
	VIRTIO_NET_F_MRG_RXBUF = 15

	// Configuration status field is available.
	VIRTIO_NET_F_STATUS = 16

	// Control channel is available.
	VIRTIO_NET_F_CTRL_VQ = 17

	// Control channel RX mode support.
	VIRTIO_NET_F_CTRL_RX = 18

	// Control channel VLAN filtering.
	VIRTIO_NET_F_CTRL_VLAN = 19

	// Control channel RX extra mode support.
	VIRTIO_NET_F_CTRL_RX_EXTRA = 20

	// Driver can send gratuitous packets.
	VIRTIO_NET_F_GUEST_ANNOUNCE = 21

	// Device supports multiqueue with automatic receive steering.
	VIRTIO_NET_F_MQ = 22

	// Set MAC address through control channel.
	VIRTIO_NET_F_CTRL_MAC_ADDR = 23

	// Device supports inner header hash for encapsulated packets.
	VIRTIO_NET_F_HASH_TUNNEL = 51

	// Device supports virtqueue notification coalescing.
	VIRTIO_NET_F_VQ_NOTF_COAL = 52

	// Device supports notifications coalescing.
	VIRTIO_NET_F_NOTF_COAL = 53

	// Driver can receive USOv4 packets.
	VIRTIO_NET_F_GUEST_USO4 = 54

	// Driver can receive USOv6 packets.
	VIRTIO_NET_F_GUEST_USO6 = 55

	// Device can receive USO packets. Unlike UFO = (fragmenting the
	// packet) the USO splits large UDP packet to several segments
	// when each of these smaller packets has UDP header.
	VIRTIO_NET_F_HOST_USO = 56

	// Device can report per-packet hash value and a type of
	// calculated hash.
	VIRTIO_NET_F_HASH_REPORT = 57

	// Driver can provide the exact hdr_len value. Device benefits
	// from knowing the exact header length.
	VIRTIO_NET_F_GUEST_HDRLEN = 59

	// Device supports RSS (receive-side scaling) with Toeplitz hash
	// calculation and configurable hash parameters for receive
	// steering.
	VIRTIO_NET_F_RSS = 60

	// Device can process duplicated ACKs and report number of
	// coalesced segments and duplicated ACKs.
	VIRTIO_NET_F_RSC_EXT = 61

	// Device may act as a standby for a primary device with the same
	// MAC address.
	VIRTIO_NET_F_STANDBY = 62

	// Device reports speed and duplex.
	VIRTIO_NET_F_SPEED_DUPLEX = 63
)

type Net struct {
	MMIO
	MAC [8]byte
}

func NewNet(hart isa.Hart, start uint64, plic *dev.PLIC, irq uint32,
	mem *memory.Memory) *Net {

	net := &Net{
		MMIO: MMIO{
			Level:    LogDebug,
			Name:     "virtio-net",
			DeviceID: NetDeviceID,
			Features: 0, //1 << VIRTIO_NET_F_MAC,
			Hart:     hart,
			Start:    start,
			End:      start + NetSize,
			Plic:     plic,
			IRQ:      irq,
			Mem:      mem,
		},
	}
	net.InitQueues(2)
	net.MMIO.Handler = net

	return net
}

// Reset implements Handler.Reset.
func (net *Net) Reset() error {
	return nil
}

var netRegs = map[uint64]string{
	0x100: "MAC[0]",
	0x101: "MAC[1]",
	0x102: "MAC[2]",
	0x103: "MAC[3]",
	0x104: "MAC[4]",
	0x105: "MAC[5]",
}

// ExecuteDescriptorChain implements Handler.ExecuteDescriptorChain.
func (net *Net) ExecuteDescriptorChain(vq *Queue, idx uint16) (uint32, error) {
	net.debugf("vq-%v: chain: idx=%v", vq.Index, idx)

	desc, err := vq.loadDesc(idx)
	if err != nil {
		return 0, err
	}
	if desc.Len < 20 {
		return 0, fmt.Errorf("truncated request: len=%v", desc.Len)
	}
	hdr, err := net.decodeHeader(desc.Addr)
	if err != nil {
		return 0, err
	}
	net.debugf("hdr: %#v", hdr)

	net.debugf("desc: %v", desc)

	return 0, fmt.Errorf("ExecuteDescriptorChain not implemented yet")
}

func (net *Net) Load8(paddr uint64) (uint8, error) {
	offset := paddr - net.Start

	reg, ok := netRegs[offset]
	if ok {
		net.debugf("Load8(%v[0x%03x])", reg, offset)
	} else {
		net.debugf("Load8(%x)", offset)
	}

	switch offset {
	// 5.1.4 Device configuration layout at offset 0x100.
	case 0x100, 0x101, 0x102, 0x103, 0x104, 0x105: // MAC.
		return net.MAC[offset-0x100], nil

	default:
		return net.MMIO.Load8(paddr)
	}
}

const (
	VIRTIO_NET_HDR_F_NEEDS_CSUM = 1
	VIRTIO_NET_HDR_F_DATA_VALID = 2
	VIRTIO_NET_HDR_F_RSC_INFO   = 4

	VIRTIO_NET_HDR_GSO_NONE   = 0
	VIRTIO_NET_HDR_GSO_TCPV4  = 1
	VIRTIO_NET_HDR_GSO_UDP    = 3
	VIRTIO_NET_HDR_GSO_TCPV6  = 4
	VIRTIO_NET_HDR_GSO_UDP_L4 = 5
	VIRTIO_NET_HDR_GSO_ECN    = 0x80
)

type NetHdr struct {
	Flags           uint8
	GSOType         uint8
	HdrLen          uint16
	GSOSize         uint16
	CSUMStart       uint16
	CSUMOffset      uint16
	NumBuffers      uint16
	HashValue       uint32
	HashReport      uint16
	PaddingReserved uint16
}

func (net *Net) decodeHeader(addr uint64) (*NetHdr, error) {
	hdr := new(NetHdr)
	var err error

	hdr.Flags, err = net.readGuestUint8(addr)
	if err != nil {
		return nil, err
	}
	hdr.GSOType, err = net.readGuestUint8(addr + 1)
	if err != nil {
		return nil, err
	}
	hdr.HdrLen, err = net.readGuestUint16(addr + 2)
	if err != nil {
		return nil, err
	}
	hdr.GSOSize, err = net.readGuestUint16(addr + 4)
	if err != nil {
		return nil, err
	}
	hdr.CSUMStart, err = net.readGuestUint16(addr + 6)
	if err != nil {
		return nil, err
	}
	hdr.CSUMOffset, err = net.readGuestUint16(addr + 8)
	if err != nil {
		return nil, err
	}
	hdr.NumBuffers, err = net.readGuestUint16(addr + 10)
	if err != nil {
		return nil, err
	}
	hdr.HashValue, err = net.readGuestUint32(addr + 12)
	if err != nil {
		return nil, err
	}
	hdr.HashReport, err = net.readGuestUint16(addr + 16)
	if err != nil {
		return nil, err
	}
	hdr.PaddingReserved, err = net.readGuestUint16(addr + 18)
	if err != nil {
		return nil, err
	}

	return hdr, nil
}
