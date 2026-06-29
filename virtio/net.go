//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package virtio

//lint:file-ignore ST1003 to match the C coding style for constants.

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strings"

	"github.com/markkurossi/riscv/dev"
	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/logger"
	"github.com/markkurossi/riscv/memory"
	"github.com/markkurossi/riscv/network"
	"github.com/markkurossi/riscv/network/dhcp"
	"github.com/markkurossi/riscv/network/tun"
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
	HostMAC  network.MAC
	GuestMAC network.MAC

	HostIP  net.IP
	GuestIP net.IP

	tun  *tun.Tunnel
	dhcp *dhcp.Server

	recvCh chan []byte

	tunReadCh  chan int
	tunReadBuf []byte

	receivedPacket []byte

	// Statistics.
	stSent uint64
	stRcvd uint64
}

func NewNet(hart isa.Hart, start uint64, plic *dev.PLIC, irq uint32,
	mem *memory.Memory, ip, gw, hostname, domainname string) (*Net, error) {

	tunnel, err := tun.Create()
	if err != nil {
		return nil, err
	}
	err = tunnel.Configure(tun.Config{
		LocalIP:  gw,
		RemoteIP: ip,
	})
	if err != nil {
		fmt.Printf("tunnel.Configure failed: %v\r\n", err)
		return nil, err
	}
	hostIP := net.ParseIP(gw).To4()
	if hostIP == nil {
		return nil, fmt.Errorf("invalid host IP address %v", gw)
	}
	guestIP := net.ParseIP(ip).To4()
	if guestIP == nil {
		return nil, fmt.Errorf("invalid guest IP address %v", ip)
	}

	vio := &Net{
		MMIO: MMIO{
			Log: logger.Log{
				Name:  "virtio-net",
				Level: logger.Info,
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
		HostIP:  hostIP,
		GuestIP: guestIP,
		tun:     tunnel,
		recvCh:  make(chan []byte),
	}
	vio.Init(2)
	vio.MMIO.Handler = vio

	_, err = rand.Read(vio.HostMAC[:])
	if err != nil {
		return nil, err
	}
	vio.HostMAC[0] = (vio.HostMAC[0] & 0xfe) | 0x02

	_, err = rand.Read(vio.GuestMAC[:])
	if err != nil {
		return nil, err
	}
	vio.GuestMAC[0] = (vio.GuestMAC[0] & 0xfe) | 0x02

	vio.dhcp = dhcp.NewServer("", hostIP)
	vio.dhcp.DomainName = domainname
	vio.dhcp.DNS = append(vio.dhcp.DNS, net.IP([]byte{
		8, 8, 8, 8,
	}))
	vio.dhcp.DNS = append(vio.dhcp.DNS, net.IP([]byte{
		1, 1, 1, 1,
	}))
	vio.dhcp.AddClient(vio.GuestMAC, &dhcp.ClientInfo{
		IP:       guestIP,
		Hostname: hostname,
	})

	vio.Infof("tunnel: %v", tunnel.Name)
	vio.Infof("guest : %v %v", vio.GuestMAC, guestIP)
	vio.Infof("host  : %v %v", vio.HostMAC, hostIP)

	go vio.receiver(vio.queues[0])
	go vio.tunReader()

	return vio, nil
}

func (vio *Net) receiver(vq *Queue) {
	vio.Debugf("receiver started for vq-%v", vq.Index)
	for {
		packet := <-vio.recvCh
		vio.Debugf("received packet of %v bytes", len(packet))

		vio.M.Lock()
		for vio.receivedPacket != nil {
			vio.C.Wait()
		}
		vio.receivedPacket = packet

		vio.ProcessQueue(vq.Index)
		vio.M.Unlock()
	}
}

func (vio *Net) tunReader() {
	for {
		ip, err := vio.tun.Read()
		if err != nil {
			vio.Errorf("tun.Read: %v", err)
			continue
		}
		// XXX Check what packet this is.

		network.DebugIP(vio, ip)

		buf := make([]byte, 12+14+len(ip))
		network.MakeEthernet(buf[12:], vio.GuestMAC, vio.HostMAC, 0x0800)
		copy(buf[12+14:], ip)
		vio.recvCh <- buf
	}
}

// Reset implements Handler.Reset.
func (vio *Net) Reset() error {
	return nil
}

// DeviceStats implements Handler.DeviceStats
func (vio *Net) DeviceStats() {
	fmt.Printf("%v: sent  : %v\n", vio.Name, FileSize(vio.stSent))
	fmt.Printf("%v: rcvd  : %v\n", vio.Name, FileSize(vio.stRcvd))
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
func (vio *Net) ExecuteDescriptorChain(vq *Queue, idx uint16) (uint32, error) {
	vio.Debugf("vq%v: chain: idx=%v", vq.Index, idx)

	if vq.Index%2 == 0 {
		return vio.processReceiveQueue(vq, idx)
	}

	// Transmit queue.

	desc, err := vq.loadDesc(idx)
	if err != nil {
		return 0, err
	}
	vio.Debugf("desc: %v", desc)

	if desc.Flags&VIRTQ_DESC_F_WRITE != 0 {
		return 0, fmt.Errorf("invalid transmitq%v flags: %x",
			vq.Index, desc.Flags)
	}

	var transferred uint32
	var hdr *NetHdr
	var payload []byte

	if desc.Len < 12 {
		return 0, fmt.Errorf("truncated request: len=%v", desc.Len)
	} else if desc.Len == 12 {
		// First descriptor is the header.
		chunk, err := vio.guestData(desc.Addr, uint64(desc.Len))
		if err != nil {
			return 0, err
		}
		hdr, err = vio.decodeHeader(chunk)
		if err != nil {
			return 0, err
		}
		transferred = 12
	} else {
		// Header and payload start in the first descriptor.
		chunk, err := vio.guestData(desc.Addr, uint64(desc.Len))
		if err != nil {
			return 0, err
		}
		hdr, err = vio.decodeHeader(chunk)
		if err != nil {
			return 0, err
		}
		payload = chunk[12:]
		transferred = desc.Len
	}
	// Check if payload is sent in multiple chunks.
	for desc.Flags&VIRTQ_DESC_F_NEXT != 0 {
		desc, err = vq.loadDesc(desc.Next)
		if err != nil {
			return 0, err
		}
		vio.Debugf("desc: %v", desc)
		if desc.Flags&VIRTQ_DESC_F_WRITE != 0 {
			return 0, fmt.Errorf("invalid transmitq%v payload flags: %x",
				vq.Index, desc.Flags)
		}
		chunk, err := vio.guestData(desc.Addr, uint64(desc.Len))
		if err != nil {
			return 0, err
		}
		payload = append(payload, chunk...)
		transferred += desc.Len
	}

	err = vio.processSend(hdr, payload)
	if err != nil {
		return 0, err
	}

	return transferred, nil
}

func (vio *Net) processReceiveQueue(vq *Queue, idx uint16) (uint32, error) {
	if vio.receivedPacket == nil {
		// No input packet yet.
		return 0, nil
	}

	// Store data into descriptor chain.
	var received uint32
	for len(vio.receivedPacket) > 0 {
		desc, err := vq.loadDesc(idx)
		if err != nil {
			return 0, err
		}
		vio.Debugf("desc: %v", desc)

		if desc.Flags&VIRTQ_DESC_F_WRITE == 0 {
			return 0, fmt.Errorf("invalid receiveq%v flags: %x",
				vq.Index, desc.Flags)
		}

		// Get next buffer.
		buf, err := vio.guestData(desc.Addr, uint64(desc.Len))
		if err != nil {
			return 0, err
		}
		n := copy(buf, vio.receivedPacket)
		received += uint32(n)
		vio.receivedPacket = vio.receivedPacket[n:]

		// Check next chunk
		for desc.Flags&VIRTQ_DESC_F_NEXT == 0 {
			break
		}
		idx = desc.Next
	}
	vio.Debugf("vq%v: transferred %v bytes, %v tail bytes ignored",
		vq.Index, received, len(vio.receivedPacket))
	vio.receivedPacket = nil

	if received > 12+14 {
		// Count only IP bytes.
		vio.stRcvd += uint64(received - 12 - 14)
	}

	vio.C.Broadcast()

	return received, nil
}

func (vio *Net) processSend(hdr *NetHdr, data []byte) error {
	vio.Debugf("process: %v", hdr)

	if len(data) < 14 {
		return fmt.Errorf("truncated Ethernet frame: len=%v", len(data))
	}

	dstMAC := network.MAC(data[0:6])
	srcMAC := network.MAC(data[6:12])
	_ = dstMAC
	_ = srcMAC

	frameType := network.BO.Uint16(data[12:])
	packet := data[14:]

	switch frameType {
	case network.EthernetIPv4:
		network.DebugIP(vio, packet)
		vio.Tracef("IPv4:\n%s", hex.Dump(packet))

		if !vio.respondIPv4(packet) {
			_, err := vio.tun.Write(packet)
			if err != nil {
				vio.Errorf("tun.Write: %v", err)
			}
			vio.stSent += uint64(len(packet))
		}

	case network.EthernetARP:
		arp, err := network.ParseARP(packet)
		if err != nil {
			return err
		}
		if arp.OPER == 1 {
			vio.Debugf("ARP  who-has %v tell %v", arp.TPA, arp.SPA)
			if arp.TPA.Equal(vio.HostIP) {
				// Respond to ARP requests.
				resp := make([]byte, 12+14+28)
				network.MakeEthernet(resp[12:], arp.SHA, vio.HostMAC, frameType)
				network.MakeARP(resp[12+14:], 2, vio.HostMAC, arp.TPA,
					arp.SHA, arp.SPA)

				vio.recvCh <- resp
			}
		} else {
			vio.Debugf("ARP  %v is-at %v", arp.SPA, arp.SHA)
		}

	case network.EthernetIPv6:
		network.DebugIP(vio, packet)
		vio.Tracef("IPv6:\n%s", hex.Dump(packet))
		_, err := vio.tun.Write(packet)
		if err != nil {
			vio.Errorf("tun.Write: %v", err)
		}
		vio.stSent += uint64(len(packet))

	default:
		vio.Debugf("Ethernet frame %04x", frameType)
		vio.Tracef("Ethernet:\n%s", hex.Dump(packet))
		_, err := vio.tun.Write(packet)
		if err != nil {
			vio.Errorf("tun.Write: %v", err)
		}
		vio.stSent += uint64(len(packet))
	}
	return nil
}

func (vio *Net) respondIPv4(packet []byte) bool {
	if len(packet) < 20 {
		return false
	}
	ihl := packet[0] & 0b111
	hdrLen := int(ihl) * 4

	proto := packet[9]
	switch proto {
	case 17: // UDP
		if len(packet) < hdrLen+8 {
			vio.Errorf("len(packet)=%v\n", len(packet))
			return false
		}
		dstPort := network.BO.Uint16(packet[hdrLen+2:])
		switch dstPort {
		case 67:
			// DHCP server.
			req, err := dhcp.Decode(packet[hdrLen+8:])
			if err != nil {
				vio.Errorf("failed to decode DHCP message: %v", err)
				return false
			}
			vio.Tracef("DHCP request data:\n%s", hex.Dump(packet))
			vio.Debugf("DHCP request: %v", req)

			resp, err := vio.dhcp.Request(req)
			if err != nil {
				vio.Errorf("DHCP response: %v", err)
				return false
			}
			if resp == nil {
				vio.Infof("ignoring DHCP request %v", req.MsgType)
				return true
			}
			var dstMAC network.MAC
			switch req.MsgType {
			case dhcp.DHCPDISCOVER:
				vio.Infof("%v from %v (%v) via %v",
					req.MsgType, req.CHAddr[:req.HLen], req.Hostname,
					vio.tun.Name)
				vio.Infof("%v on %v to %v (%v) via %v",
					resp.MsgType, resp.YIAddr, resp.CHAddr[:resp.HLen],
					req.Hostname, vio.tun.Name)
				dstMAC = network.MAC([]byte{
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				})

			case dhcp.DHCPREQUEST:
				vio.Infof("%v for %v (%v) from %v (%v) via %v",
					req.MsgType, req.AddressRequest, vio.HostIP,
					req.CHAddr[:req.HLen], req.Hostname,
					vio.tun.Name)
				vio.Infof("%v on %v to %v (%v) via %v",
					resp.MsgType, resp.YIAddr, resp.CHAddr[:resp.HLen],
					req.Hostname, vio.tun.Name)
				dstMAC = vio.GuestMAC
			}

			vio.Debugf("DHCP response: %v", resp)
			rdata := resp.Encode()

			data := make([]byte, 12+14+20+8+len(rdata))

			network.MakeEthernet(data[12:],
				dstMAC, vio.HostMAC, network.EthernetIPv4)

			// IP.
			ip := 12 + 14
			data[ip+0] = 0x45
			data[ip+1] = 0x10
			network.BO.PutUint16(data[ip+2:], uint16(20+8+len(rdata)))
			data[ip+8] = 0x80 // TTL
			data[ip+9] = 0x11 // UDP
			copy(data[ip+12:], vio.HostIP)
			copy(data[ip+16:], network.BroadcastIP)
			network.ComputeChecksum(data[ip:])

			// UDP.
			network.BO.PutUint16(data[ip+20:], 67)
			network.BO.PutUint16(data[ip+22:], 68)
			network.BO.PutUint16(data[ip+24:], uint16(8+len(rdata)))
			copy(data[ip+28:], rdata)
			network.ComputeUDPChecksum(data[ip:])

			vio.Tracef("DHCP response data:\n%s", hex.Dump(data[ip:]))

			vio.recvCh <- data
			return true

		default:
			vio.Debugf("skipping UDP port %v", dstPort)
		}

	default:
		vio.Debugf("skipping protocol %v", proto)
	}

	return false
}

func (vio *Net) Load8(paddr uint64) (uint8, error) {
	offset := paddr - vio.Start

	reg, ok := netRegs[offset]
	if ok {
		vio.Debugf("Load8(%v[0x%03x])", reg, offset)
	} else {
		vio.Debugf("Load8(%x)", offset)
	}

	switch offset {
	// 5.1.4 Device configuration layout at offset 0x100.
	case 0x100, 0x101, 0x102, 0x103, 0x104, 0x105: // MAC.
		return vio.GuestMAC[offset-0x100], nil

	default:
		return vio.MMIO.Load8(paddr)
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

func (vio *Net) decodeHeader(data []byte) (*NetHdr, error) {
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
