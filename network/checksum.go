//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package network

import (
	"encoding/hex"
	"fmt"
)

func ComputeChecksum(packet []byte) uint16 {
	if len(packet) < 20 {
		return 0
	}
	var sum uint32
	for i := 0; i < 10; i++ {
		if i == 5 {
			// Skip checksum.
			continue
		}
		sum += uint32(BO.Uint16(packet[i*2:]))
	}
	sum += sum >> 16
	checksum := uint16(^sum)
	fmt.Printf("checksum: %x\n", checksum)

	return checksum
}

func VerifyChecksum(packet []byte) bool {
	if len(packet) < 20 {
		return false
	}
	var sum uint32
	for i := 0; i < 10; i++ {
		sum += uint32(BO.Uint16(packet[i*2:]))
	}
	sum += sum >> 16
	checksum := uint16(^sum)
	fmt.Printf("checksum: %x\n", checksum)

	return checksum == 0
}

func makePseudoHeader(hdr *[12]byte, packet []byte) (int, int) {
	copy(hdr[0:4], packet[12:16])
	copy(hdr[4:8], packet[16:20])
	hdr[9] = packet[9]

	ihl := packet[0] & 0b111
	hdrLen := int(ihl) * 4
	payloadLen := int(BO.Uint16(packet[2:])) - hdrLen
	BO.PutUint16(hdr[10:], uint16(payloadLen))

	return hdrLen, payloadLen
}

func ComputeUDPChecksum(packet []byte) uint16 {
	if len(packet) < 20+8 {
		return 0
	}

	var hdr [12]byte
	hdrLen, payloadLen := makePseudoHeader(&hdr, packet)

	var sum uint32

	// Pseudo-header.
	for i := 0; i < 6; i++ {
		sum += uint32(BO.Uint16(hdr[i*2:]))
	}

	// Payload.
	payload := packet[hdrLen:]
	for i := 0; i*2 < payloadLen; i++ {
		if i == 3 {
			// Skip checksum.
			continue
		}
		var word uint16

		if i*2+1 >= payloadLen {
			word = uint16(payload[i*2]) << 8
		} else {
			word = BO.Uint16(payload[i*2:])
		}

		sum += uint32(word)
	}
	sum += sum >> 16
	checksum := uint16(^sum)
	fmt.Printf("UDP checksum: %x\n", checksum)

	return checksum

}

func VerifyUDPChecksum(packet []byte) bool {
	if len(packet) < 20+8 {
		return false

	}
	var hdr [12]byte
	hdrLen, payloadLen := makePseudoHeader(&hdr, packet)
	fmt.Printf("UDP pseudo-header:\n%s", hex.Dump(hdr[:]))

	var sum uint32

	// Pseudo-header.
	for i := 0; i < 6; i++ {
		sum += uint32(BO.Uint16(hdr[i*2:]))
	}

	// Payload.
	payload := packet[hdrLen:]
	for i := 0; i*2 < payloadLen; i++ {
		var word uint16

		if i*2+1 >= payloadLen {
			word = uint16(payload[i*2]) << 8
		} else {
			word = BO.Uint16(payload[i*2:])
		}

		sum += uint32(word)
	}
	sum += sum >> 16
	checksum := uint16(^sum)
	fmt.Printf("UDP checksum: %x\n", checksum)

	return checksum == 0
}
