//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package network

func Checksum(packet []byte) uint16 {
	if len(packet) < 20 {
		return 0
	}
	var sum uint32
	for i := 0; i < 10; i++ {
		sum += uint32(BO.Uint16(packet[i*2:]))
	}
	sum += sum >> 16
	checksum := uint16(^sum)

	return checksum
}

func ComputeChecksum(packet []byte) bool {
	if len(packet) < 20 {
		return false
	}
	BO.PutUint16(packet[10:], 0)
	BO.PutUint16(packet[10:], Checksum(packet))

	return true
}

func VerifyChecksum(packet []byte) bool {
	if len(packet) < 20 {
		return false
	}
	return Checksum(packet) == 0
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

func UDPChecksum(packet []byte) uint16 {
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

	return checksum
}

func ComputeUDPChecksum(packet []byte) bool {
	if len(packet) < 20+8 {
		return false
	}

	ihl := packet[0] & 0b111
	hdrLen := int(ihl) * 4

	BO.PutUint16(packet[hdrLen+6:], 0)
	BO.PutUint16(packet[hdrLen+6:], UDPChecksum(packet))

	return true
}

func VerifyUDPChecksum(packet []byte) bool {
	if len(packet) < 20+8 {
		return false

	}

	return UDPChecksum(packet) == 0
}
