//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package network

// MakeEthernet creates Ethernet packet with the argument data.
func MakeEthernet(buf []byte, dst, src MAC, frameType uint16) {
	copy(buf[0:16], dst[:])
	copy(buf[6:12], src[:])
	BO.PutUint16(buf[12:], frameType)
}
