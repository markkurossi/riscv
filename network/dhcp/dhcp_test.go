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
	"testing"
)

var dhcpPackets = []string{
	`00000000  45 10 01 48 00 00 00 00  80 11 39 96 00 00 00 00  |E..H......9.....|
00000010  ff ff ff ff 00 44 00 43  01 34 7c ad 01 01 06 00  |.....D.C.4|.....|
00000020  29 a5 fc 68 00 34 00 00  00 00 00 00 00 00 00 00  |)..h.4..........|
00000030  00 00 00 00 00 00 00 00  fe 53 c5 87 31 93 00 00  |.........S..1...|
00000040  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000050  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000060  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000070  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000080  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000090  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
000000a0  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
000000b0  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
000000c0  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
000000d0  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
000000e0  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
000000f0  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000100  00 00 00 00 00 00 00 00  63 82 53 63 35 01 01 3d  |........c.Sc5..=|
00000110  07 01 fe 53 c5 87 31 93  0c 07 66 72 65 65 62 73  |...S..1...freebs|
00000120  64 37 0a 01 1c 02 79 03  0f 06 0c 77 1a ff 00 00  |d7....y....w....|
00000130  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000140  00 00 00 00 00 00 00 00                           |........|
`,
}

func TestDHCPDecode(t *testing.T) {
	server := NewDHCPServer("markkurossi.com", net.IP([]byte{
		192, 168, 42, 254,
	}))
	server.AddClient(MAC([]byte{
		0xfe, 0x53, 0xc5, 0x87, 0x31, 0x93}),
		&ClientInfo{
			IP: net.IP([]byte{192, 168, 42, 1}),
		})

	for idx, packet := range dhcpPackets {
		data, err := Parse([]byte(packet))
		if err != nil {
			t.Errorf("DHCP-%v: failed to parse packet: %v", idx, err)
			continue
		}
		fmt.Printf("IP:\n%s", hex.Dump(data[:20]))
		fmt.Printf("UDP:\n%s", hex.Dump(data[20:28]))

		req, err := DecodeDHCP(data[28:])
		if err != nil {
			t.Errorf("DecodeDHCP-%v: %v", idx, err)
			continue
		}
		fmt.Printf("DHCP:%v:0: %v\n", idx, req)
		if req.Cookie != DHCPOptionsCookie {
			t.Errorf("DHCP-%v: invalid cookie %x, expected %x",
				idx, req.Cookie, DHCPOptionsCookie)
		}

		resp, err := server.Offer(req)
		if err != nil {
			t.Fatalf("DHCP-%v: failed to create DHCPOFFER: %v", idx, err)
		}
		fmt.Printf("DHCP:%v:1: %v\n", idx, resp)
	}

}
