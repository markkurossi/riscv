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
	"net"
	"strings"

	"github.com/markkurossi/riscv/dev"
	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/logger"
	"github.com/markkurossi/riscv/memory"
	"github.com/markkurossi/riscv/tun"
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

	HostIP  net.IP
	GuestIP net.IP

	tun *tun.Tunnel

	localCh chan []byte

	tunReadCh  chan int
	tunReadBuf []byte

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
	mem *memory.Memory, ip, gw string) (*Net, error) {

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
	hostIP := net.ParseIP(gw)
	if hostIP == nil {
		return nil, fmt.Errorf("invalid host IP address %v", gw)
	}
	guestIP := net.ParseIP(ip)
	if guestIP == nil {
		return nil, fmt.Errorf("invalid guest IP address %v", ip)
	}

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
		HostIP:  hostIP,
		GuestIP: guestIP,
		tun:     tunnel,
		localCh: make(chan []byte),
	}
	net.Init(2)
	net.MMIO.Handler = net

	_, err = rand.Read(net.HostMAC[:])
	if err != nil {
		return nil, err
	}
	net.HostMAC[0] = (net.HostMAC[0] & 0xfe) | 0x02

	_, err = rand.Read(net.GuestMAC[:])
	if err != nil {
		return nil, err
	}
	net.GuestMAC[0] = (net.GuestMAC[0] & 0xfe) | 0x02

	net.Infof("tunnel   : %v", tunnel.Name)
	net.Infof("guest IP : %v", ip)
	net.Infof("host IP  : %v", gw)
	net.Infof("guest MAC: %v", net.GuestMAC)
	net.Infof("host MAC : %v", net.HostMAC)

	go net.receiver(net.queues[0])
	go net.tunReader()

	return net, nil
}

func (vio *Net) receiver(vq *Queue) {
	vio.Debugf("receiver started for vq-%v", vq.Index)
	for {
		local := <-vio.localCh
		vio.Debugf("sending local packet of %v bytes", len(local))

		vio.M.Lock()
		for vio.receiveBuf == nil {
			vio.C.Wait()
		}
		vio.received = uint32(copy(vio.receiveBuf, local))
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

		vio.debugIP(ip)

		buf := make([]byte, 12+14+len(ip))
		makeEthernet(buf[12:], vio.GuestMAC, vio.HostMAC, 0x0800)
		copy(buf[12+14:], ip)
		vio.localCh <- buf
	}
}

// Reset implements Handler.Reset.
func (vio *Net) Reset() error {
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
	desc, err := vq.loadDesc(idx)
	if err != nil {
		return 0, err
	}
	vio.Debugf("desc: %v", desc)

	if desc.Flags&VIRTQ_DESC_F_WRITE == 0 {
		return 0, fmt.Errorf("invalid receiveq%v flags: %x",
			vq.Index, desc.Flags)
	}
	// XXX VIRTQ_DESC_F_NEXT
	if vio.received > 0 {
		// We have received data to new.receiveBuf.
		if vio.receiveIdx != idx {
			return 0, fmt.Errorf("receive queue out-of-sync: %v vs %v",
				vio.receiveIdx, idx)
		}
		received := vio.received
		vio.receiveIdx = 0
		vio.receiveBuf = nil
		vio.received = 0
		return received, nil
	}

	// Save next buffer.
	buf, err := vio.guestData(desc.Addr, uint64(desc.Len))
	if err != nil {
		return 0, err
	}
	vio.receiveIdx = idx
	vio.receiveBuf = buf
	vio.received = 0

	vio.C.Broadcast()

	return 0, nil
}

func (vio *Net) processSend(hdr *NetHdr, data []byte) error {
	vio.Debugf("process: %v", hdr)

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
		vio.debugIP(data[14:])
		vio.Tracef("%s", hex.Dump(data[14:]))
		_, err := vio.tun.Write(data[14:])
		if err != nil {
			vio.Errorf("tun.Write: %v", err)
		}

	case 0x0806:
		arp, err := parseARP(data[14:])
		if err != nil {
			return err
		}
		if arp.OPER == 1 {
			vio.Debugf("ARP  who-has %v tell %v", arp.TPA, arp.SPA)
			if arp.TPA.Equal(vio.HostIP) {
				// Respond to ARP requests.
				resp := make([]byte, 12+14+28)
				makeEthernet(resp[12:], arp.SHA, vio.HostMAC, frameType)
				makeARP(resp[12+14:], 2, vio.HostMAC, arp.TPA, arp.SHA, arp.SPA)

				vio.localCh <- resp
			}
		} else {
			vio.Debugf("ARP  %v is-at %v", arp.SPA, arp.SHA)
		}

	case 0x86dd:
		vio.debugIP(data[14:])
		vio.Tracef("%s", hex.Dump(data[14:]))
		_, err := vio.tun.Write(data[14:])
		if err != nil {
			vio.Errorf("tun.Write: %v", err)
		}

	default:
		vio.Debugf("Ethernet frame %04x", frameType)
		vio.Tracef("%s", hex.Dump(data[14:]))
		_, err := vio.tun.Write(data[14:])
		if err != nil {
			vio.Errorf("tun.Write: %v", err)
		}
	}
	return nil
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

type ARP struct {
	HTYPE uint16
	PTYPE uint16
	HLEN  uint8
	PLEN  uint8
	OPER  uint16
	SHA   MAC
	SPA   net.IP
	THA   MAC
	TPA   net.IP
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
		SPA:   net.IP(data[14:18]),
		THA:   MAC(data[18:24]),
		TPA:   net.IP(data[24:28]),
	}, nil
}

func makeARP(buf []byte, oper uint16, sha MAC, spa net.IP,
	tha MAC, tpa net.IP) {
	netBO.PutUint16(buf[0:], 1)
	netBO.PutUint16(buf[2:], 0x800)
	buf[4] = 6
	buf[5] = 4
	netBO.PutUint16(buf[6:], oper)
	copy(buf[8:14], sha[:])
	copy(buf[14:18], spa)
	copy(buf[18:24], tha[:])
	copy(buf[24:28], tpa)
}

func makeEthernet(buf []byte, dst, src MAC, frameType uint16) {
	copy(buf[0:16], dst[:])
	copy(buf[6:12], src[:])
	netBO.PutUint16(buf[12:], frameType)
}

var ipProtoNames = map[byte]string{
	0:   "HOPOPT",
	1:   "ICMP",
	2:   "IGMP",
	3:   "GGP",
	4:   "IP-in-IP",
	5:   "ST",
	6:   "TCP",
	7:   "CBT",
	8:   "EGP",
	9:   "IGP",
	10:  "BBN-RCC-MON",
	11:  "NVP-II",
	12:  "PUP",
	13:  "ARGUS",
	14:  "EMCON",
	15:  "XNET",
	16:  "CHAOS",
	17:  "UDP",
	18:  "MUX",
	19:  "DCN-MEAS",
	20:  "HMP",
	21:  "PRM",
	22:  "XNS-IDP",
	23:  "TRUNK-1",
	24:  "TRUNK-2",
	25:  "LEAF-1",
	26:  "LEAF-2",
	27:  "RDP",
	28:  "IRTP",
	29:  "ISO-TP4",
	30:  "NETBLT",
	31:  "MFE-NSP",
	32:  "MERIT-INP",
	33:  "DCCP",
	34:  "3PC",
	35:  "IDPR",
	36:  "XTP",
	37:  "DDP",
	38:  "IDPR-CMTP",
	39:  "TP++",
	40:  "IL",
	41:  "IPv6",
	42:  "SDRP",
	43:  "IPv6-Route",
	44:  "IPv6-Frag",
	45:  "IDRP",
	46:  "RSVP",
	47:  "GRE",
	48:  "DSR",
	49:  "BNA",
	50:  "ESP",
	51:  "AH",
	52:  "I-NLSP",
	53:  "SwIPe",
	54:  "NARP",
	55:  "MOBILE",
	56:  "TLSP",
	57:  "SKIP",
	58:  "IPv6-ICMP",
	59:  "IPv6-NoNxt",
	60:  "IPv6-Opts",
	62:  "CFTP",
	64:  "SAT-EXPAK",
	65:  "KRYPTOLAN",
	66:  "RVD",
	67:  "IPPC",
	69:  "SAT-MON",
	70:  "VISA",
	71:  "IPCU",
	72:  "CPNX",
	73:  "CPHB",
	74:  "WSN",
	75:  "PVP",
	76:  "BR-SAT-MON",
	77:  "SUN-ND",
	78:  "WB-MON",
	79:  "WB-EXPAK",
	80:  "ISO-IP",
	81:  "VMTP",
	82:  "SECURE-VMTP",
	83:  "VINES",
	84:  "IPTM",
	85:  "NSFNET-IGP",
	86:  "DGP",
	87:  "TCF",
	88:  "EIGRP",
	89:  "OSPF",
	90:  "Sprite-RPC",
	91:  "LARP",
	92:  "MTP",
	93:  "AX.25",
	94:  "OS",
	95:  "MICP",
	96:  "SCC-SP",
	97:  "ETHERIP",
	98:  "ENCAP",
	100: "GMTP",
	101: "IFMP",
	102: "PNNI",
	103: "PIM",
	104: "ARIS",
	105: "SCPS",
	106: "QNX",
	107: "A/N",
	108: "IPComp",
	109: "SNP",
	110: "Compaq-Peer",
	111: "IPX-in-IP",
	112: "VRRP",
	113: "PGM",
	115: "L2TP",
	116: "DDX",
	117: "IATP",
	118: "STP",
	119: "SRP",
	120: "UTI",
	121: "SMP",
	122: "SM",
	123: "PTP",
	124: "IS-IS over IPv4",
	125: "FIRE",
	126: "CRTP",
	127: "CRUDP",
	128: "SSCOPMCE",
	129: "IPLT",
	130: "SPS",
	131: "PIPE",
	132: "SCTP",
	133: "FC",
	134: "RSVP-E2E-IGNORE",
	135: "Mobility Header",
	136: "UDPLite",
	137: "MPLS-in-IP",
	138: "manet",
	139: "HIP",
	140: "Shim6",
	141: "WESP",
	142: "ROHC",
	143: "Ethernet",
	144: "AGGFRAG",
	145: "NSH",
	146: "Homa",
	147: "BIT-EMU",
}

func ipProtoName(proto uint8) string {
	name, ok := ipProtoNames[proto]
	if ok {
		return name
	}
	return fmt.Sprintf("%02x", proto)
}

var icmpTypeNames = map[byte]string{
	0:  "echo reply",
	3:  "destination unreachable",
	4:  "source quench",
	5:  "redirect message",
	8:  "echo",
	9:  "router advertisement",
	10: "router solicitation",
	11: "time exceeded",
	12: "parameter problem",
	13: "timestamp",
	14: "timestamp reply",
	15: "information request",
	16: "information reply",
	17: "address mask request",
	18: "address mask reply",
	30: "traceroute",
	42: "extended echo",
	43: "extended echo reply",
}

func icmpTypeName(t byte) string {
	name, ok := icmpTypeNames[t]
	if ok {
		return name
	}
	return fmt.Sprintf("%02x", t)
}

func (vio *Net) debugIP(packet []byte) int {
	if len(packet) < 20 {
		vio.Errorf("truncated IP packet:\n%s", hex.Dump(packet))
		return 0
	}

	v := int(packet[0] >> 4)

	var hdrLen int
	var proto uint8
	var src, dst net.IP

	switch v {
	case 4:
		ihl := packet[0] & 0b111
		hdrLen = int(ihl) * 4
		proto = packet[9]
		src = net.IP(packet[12:16])
		dst = net.IP(packet[16:20])

	case 6:
		hdrLen = 40
		if len(packet) < hdrLen {
			vio.Errorf("truncated IPv6 packet:\n%s", hex.Dump(packet))
		}
		proto = packet[6]
		src = net.IP(packet[8:24])
		dst = net.IP(packet[24:40])

	default:
		vio.Errorf("invalid IP packet: version=%v:\n%s", v, hex.Dump(packet))
		return 0
	}

	var srcPort, dstPort uint16
	if len(packet) >= hdrLen+4 {
		srcPort = netBO.Uint16(packet[hdrLen:])
		dstPort = netBO.Uint16(packet[hdrLen+2:])
	}

	switch proto {
	case 1: // ICMP
		vio.Debugf("ICMP %v -> %v %v", src, dst, icmpTypeName(packet[hdrLen]))

	case 6, 17: // TCP, UDP
		vio.Debugf("%s %v:%v -> %v:%v",
			ipProtoName(proto), src, srcPort, dst, dstPort)

	default:
		vio.Debugf("%s %v -> %v", ipProtoName(proto), src, dst)
	}

	return v
}
