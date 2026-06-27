//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package network

const (
	EthernetIPv4 uint16 = 0x0800
	EthernetARP  uint16 = 0x0806
	EthernetIPv6 uint16 = 0x86dd
)

// MakeEthernet creates Ethernet packet with the argument data.
func MakeEthernet(buf []byte, dst, src MAC, frameType uint16) {
	copy(buf[0:16], dst[:])
	copy(buf[6:12], src[:])
	BO.PutUint16(buf[12:], frameType)
}
