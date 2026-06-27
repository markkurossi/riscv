//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package network

import (
	"encoding/hex"
	"fmt"
	"net"

	"github.com/markkurossi/riscv/logger"
)

var (
	ZeroIP      = net.IP([]byte{0, 0, 0, 0})
	BroadcastIP = net.IP([]byte{255, 255, 255, 255})
	ClassCMask  = net.IP([]byte{255, 255, 255, 0})
)

func IsZeroIP(ip net.IP) bool {
	return ip == nil || ip.IsUnspecified()
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

// DebugIP verifies the IP packet and logs it to logger. The function
// returns the packet IP version.
func DebugIP(log logger.Logger, packet []byte) int {
	if len(packet) < 20 {
		log.Errorf("truncated IP packet:\n%s", hex.Dump(packet))
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
			log.Errorf("truncated IPv6 packet:\n%s", hex.Dump(packet))
		}
		proto = packet[6]
		src = net.IP(packet[8:24])
		dst = net.IP(packet[24:40])

	default:
		log.Errorf("invalid IP packet: version=%v:\n%s", v, hex.Dump(packet))
		return 0
	}

	var srcPort, dstPort uint16
	if len(packet) >= hdrLen+4 {
		srcPort = BO.Uint16(packet[hdrLen:])
		dstPort = BO.Uint16(packet[hdrLen+2:])
	}

	switch proto {
	case 1: // ICMP
		log.Debugf("ICMP %v -> %v %v", src, dst, icmpTypeName(packet[hdrLen]))

	case 6, 17: // TCP, UDP
		log.Debugf("%s %v:%v -> %v:%v",
			ipProtoName(proto), src, srcPort, dst, dstPort)

	default:
		log.Debugf("%s %v -> %v", ipProtoName(proto), src, dst)
	}

	return v
}
