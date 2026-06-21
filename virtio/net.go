//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package virtio

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/markkurossi/riscv/dev"
	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/logger"
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
	HostMAC  MAC
	GuestMAC MAC

	localCh chan []byte

	receiveIdx uint16
	receiveBuf []byte
	received   uint32
}

type MAC [6]byte

func (mac MAC) String() string {
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
}

func NewNet(hart isa.Hart, start uint64, plic *dev.PLIC, irq uint32,
	mem *memory.Memory) *Net {

	net := &Net{
		MMIO: MMIO{
			Logger: logger.Logger{
				Name:  "virtio-net",
				Level: logger.Debug,
			},
			DeviceID: NetDeviceID,
			Features: 1 << VIRTIO_NET_F_MAC,
			Hart:     hart,
			Start:    start,
			End:      start + NetSize,
			Plic:     plic,
			IRQ:      irq,
			Mem:      mem,
		},
		localCh: make(chan []byte),
	}
	net.Init(2)
	net.MMIO.Handler = net

	_, err := rand.Read(net.HostMAC[:])
	if err != nil {
		panic(err)
	}
	net.HostMAC[0] = (net.HostMAC[0] & 0xfe) | 0x02

	_, err = rand.Read(net.GuestMAC[:])
	if err != nil {
		panic(err)
	}
	net.GuestMAC[0] = (net.GuestMAC[0] & 0xfe) | 0x02

	net.Debugf("guest MAC: %v", net.GuestMAC)
	net.Debugf("host MAC : %v", net.HostMAC)

	go net.receiver(net.queues[0])

	return net
}

func (net *Net) receiver(vq *Queue) {
	net.Debugf("receiver started for vq-%v", vq.Index)
	for {
		local := <-net.localCh
		net.M.Lock()
		for net.receiveBuf == nil {
			net.C.Wait()
		}
		net.Debugf("sending local packet of %v bytes", len(local))
		net.received = uint32(copy(net.receiveBuf, local))
		net.ProcessQueue(vq.Index)
		net.M.Unlock()
	}
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

// FreeBSD sends packets in parts:
//
// 	virtio-net: DEBUG: ProcessQueue(1)
// 	virtio-net: DEBUG: vq-1: chain: idx=0
// 	virtio-net: DEBUG: desc: Buf=12@809ce300,Flags=1,Next=1
// 	virtio-net: DEBUG: process: NetHdr: flags=, gso_type=none, hdr_len=0, gso_size=0, csum_start=0, csum_offset=0, num_buffers=0
// 	virtio-net: ERROR: execute descriptor chain: truncated Ethernet frame: len=0

// ExecuteDescriptorChain implements Handler.ExecuteDescriptorChain.
func (net *Net) ExecuteDescriptorChain(vq *Queue, idx uint16) (uint32, error) {
	net.Debugf("vq-%v: chain: idx=%v", vq.Index, idx)

	desc, err := vq.loadDesc(idx)
	if err != nil {
		return 0, err
	}
	net.Debugf("desc: %v", desc)

	if desc.Flags&VIRTQ_DESC_F_WRITE != 0 {
		// Receive queue.
		if net.received > 0 {
			// We have received data to new.receiveBuf.
			if net.receiveIdx != idx {
				return 0, fmt.Errorf("receive queue out-of-sync: %v vs %v",
					net.receiveIdx, idx)
			}
			received := net.received
			net.receiveIdx = 0
			net.receiveBuf = nil
			net.received = 0
			return received, nil
		}

		// Save next buffer.
		buf, err := net.guestData(desc.Addr, uint64(desc.Len))
		if err != nil {
			return 0, err
		}
		net.receiveIdx = idx
		net.receiveBuf = buf
		net.received = 0

		net.C.Broadcast()

		return 0, nil
	}

	// Transmit queue.
	if desc.Len < 12 {
		return 0, fmt.Errorf("truncated request: len=%v", desc.Len)
	}
	buf, err := net.guestData(desc.Addr, uint64(desc.Len))
	if err != nil {
		return 0, err
	}
	hdr, err := net.decodeHeader(buf)
	if err != nil {
		return 0, err
	}
	err = net.processSend(hdr, buf[12:])
	if err != nil {
		return 0, err
	}
	return desc.Len, nil
}

func (net *Net) processSend(hdr *NetHdr, data []byte) error {
	net.Debugf("process: %v", hdr)

	if len(data) < 14 {
		return fmt.Errorf("truncated Ethernet frame: len=%v", len(data))
	}

	dstMAC := MAC(data[0:6])
	srcMAC := MAC(data[6:12])
	_ = dstMAC
	_ = srcMAC

	frameType := netBO.Uint16(data[12:])

	switch frameType {
	case 0x0800:
		net.Debugf("IPv4:\n%s", hex.Dump(data[14:]))

	case 0x0806:
		arp, err := parseARP(data[14:])
		if err != nil {
			return err
		}
		net.Debugf("ARP: %v", arp)
		if arp.OPER == 1 {
			// Respond to all ARP requests.
			resp := make([]byte, 12+14+28)
			makeEthernet(resp[12:], arp.SHA, net.HostMAC, frameType)
			makeARP(resp[12+14:], 2, net.HostMAC, arp.TPA, arp.SHA, arp.SPA)

			net.localCh <- resp
		}

	case 0x86dd:
		net.Debugf("IPv6:\n%s", hex.Dump(data[14:]))

	default:
		net.Debugf("unknown %04x:\n%s", frameType, hex.Dump(data[14:]))
	}
	return nil
}

func (net *Net) Load8(paddr uint64) (uint8, error) {
	offset := paddr - net.Start

	reg, ok := netRegs[offset]
	if ok {
		net.Debugf("Load8(%v[0x%03x])", reg, offset)
	} else {
		net.Debugf("Load8(%x)", offset)
	}

	switch offset {
	// 5.1.4 Device configuration layout at offset 0x100.
	case 0x100, 0x101, 0x102, 0x103, 0x104, 0x105: // MAC.
		return net.GuestMAC[offset-0x100], nil

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
	Flags      uint8
	GSOType    uint8
	HdrLen     uint16
	GSOSize    uint16
	CSUMStart  uint16
	CSUMOffset uint16
	NumBuffers uint16
}

func (hdr *NetHdr) String() string {
	result := "NetHdr: flags="

	var v []string
	if hdr.Flags&VIRTIO_NET_HDR_F_NEEDS_CSUM != 0 {
		v = append(v, "NEEDS_CSUM")
	}
	if hdr.Flags&VIRTIO_NET_HDR_F_DATA_VALID != 0 {
		v = append(v, "DATA_VALID")
	}
	if hdr.Flags&VIRTIO_NET_HDR_F_RSC_INFO != 0 {
		v = append(v, "RSC_INFO")
	}
	result += strings.Join(v, ",")

	result += ", gso_type="
	switch hdr.GSOType {
	case VIRTIO_NET_HDR_GSO_NONE:
		result += "none"
	case VIRTIO_NET_HDR_GSO_TCPV4:
		result += "TCPV4"
	case VIRTIO_NET_HDR_GSO_UDP:
		result += "UDP"
	case VIRTIO_NET_HDR_GSO_TCPV6:
		result += "TCP6"
	case VIRTIO_NET_HDR_GSO_UDP_L4:
		result += "UDP_L4"
	case VIRTIO_NET_HDR_GSO_ECN:
		result += "GSO_ECN"
	}
	result += fmt.Sprintf(", hdr_len=%v, gso_size=%v, csum_start=%v, csum_offset=%v, num_buffers=%v",
		hdr.HdrLen, hdr.GSOSize, hdr.CSUMStart, hdr.CSUMOffset,
		hdr.NumBuffers)

	return result
}

func (net *Net) decodeHeader(data []byte) (*NetHdr, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("invalid transmit packet: len=%v", len(data))
	}

	return &NetHdr{
		Flags:      data[0],
		GSOType:    data[1],
		HdrLen:     vioBO.Uint16(data[2:]),
		GSOSize:    vioBO.Uint16(data[4:]),
		CSUMStart:  vioBO.Uint16(data[6:]),
		CSUMOffset: vioBO.Uint16(data[8:]),
		NumBuffers: vioBO.Uint16(data[10:]),
	}, nil
}

type IPv4Addr uint32

func (ipv4 IPv4Addr) String() string {
	return fmt.Sprintf("%v.%v.%v.%v",
		byte(ipv4>>24), byte(ipv4>>16), byte(ipv4>>8), byte(ipv4))
}

type ARP struct {
	HTYPE uint16
	PTYPE uint16
	HLEN  uint8
	PLEN  uint8
	OPER  uint16
	SHA   MAC
	SPA   IPv4Addr
	THA   MAC
	TPA   IPv4Addr
}

func (arp *ARP) String() string {
	return fmt.Sprintf("HTYPE=%04x, PTYPE=%04x, HLEN=%v, PLEN=%v, OPER=%v, SHA=%v, SPA=%v, THA=%v, TPA=%v",
		arp.HTYPE, arp.PTYPE, arp.HLEN, arp.PLEN, arp.OPER,
		arp.SHA, arp.SPA, arp.THA, arp.TPA)
}

func parseARP(data []byte) (*ARP, error) {
	if len(data) != 28 {
		return nil, fmt.Errorf("invalid ARP packet: len=%v", len(data))
	}

	return &ARP{
		HTYPE: netBO.Uint16(data[0:]),
		PTYPE: netBO.Uint16(data[2:]),
		HLEN:  data[4],
		PLEN:  data[5],
		OPER:  netBO.Uint16(data[6:]),
		SHA:   MAC(data[8:14]),
		SPA:   IPv4Addr(netBO.Uint32(data[14:])),
		THA:   MAC(data[18:24]),
		TPA:   IPv4Addr(netBO.Uint32(data[24:])),
	}, nil
}

func makeARP(buf []byte, oper uint16, sha MAC, spa IPv4Addr,
	tha MAC, tpa IPv4Addr) {
	netBO.PutUint16(buf[0:], 1)
	netBO.PutUint16(buf[2:], 0x800)
	buf[4] = 6
	buf[5] = 4
	netBO.PutUint16(buf[6:], oper)
	copy(buf[8:14], sha[:])
	netBO.PutUint32(buf[14:], uint32(spa))
	copy(buf[18:24], tha[:])
	netBO.PutUint32(buf[24:], uint32(tpa))
}

func makeEthernet(buf []byte, dst, src MAC, frameType uint16) {
	copy(buf[0:16], dst[:])
	copy(buf[6:12], src[:])
	netBO.PutUint16(buf[12:], frameType)
}
