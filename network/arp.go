//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package network

import (
	"fmt"
	"net"
)

// ARP defines address resolution protocol (ARP) messages.
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

// ParseARP parses ARP package.
func ParseARP(data []byte) (*ARP, error) {
	if len(data) != 28 {
		return nil, fmt.Errorf("invalid ARP packet: len=%v", len(data))
	}

	return &ARP{
		HTYPE: BO.Uint16(data[0:]),
		PTYPE: BO.Uint16(data[2:]),
		HLEN:  data[4],
		PLEN:  data[5],
		OPER:  BO.Uint16(data[6:]),
		SHA:   MAC(data[8:14]),
		SPA:   net.IP(data[14:18]),
		THA:   MAC(data[18:24]),
		TPA:   net.IP(data[24:28]),
	}, nil
}

// MakeARP creates ARP packet with the argument data.
func MakeARP(buf []byte, oper uint16, sha MAC, spa net.IP,
	tha MAC, tpa net.IP) {
	BO.PutUint16(buf[0:], 1)
	BO.PutUint16(buf[2:], 0x800)
	buf[4] = 6
	buf[5] = 4
	BO.PutUint16(buf[6:], oper)
	copy(buf[8:14], sha[:])
	copy(buf[14:18], spa)
	copy(buf[18:24], tha[:])
	copy(buf[24:28], tpa)
}
