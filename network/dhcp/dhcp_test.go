//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package dhcp

import (
	"encoding/hex"
	"fmt"
	"net"
	"testing"

	"github.com/markkurossi/riscv/network"
	"github.com/markkurossi/text/hexdump"
)

var dhcpPackets = []string{
	`00000000  ff ff ff ff ff ff 36 8c  76 8e 5e 96 08 00 45 c0  |......6.v.^...E.|
00000010  01 40 00 00 00 00 40 11  78 ee 00 00 00 00 ff ff  |.@....@.x.......|
00000020  ff ff 00 44 00 43 01 2c  47 1b 01 01 06 00 c8 33  |...D.C.,G......3|
00000030  c1 eb 00 01 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000040  00 00 00 00 00 00 36 8c  76 8e 5e 96 00 00 00 00  |......6.v.^.....|
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
00000100  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000110  00 00 00 00 00 00 63 82  53 63 35 01 01 3d 13 ff  |......c.Sc5..=..|
00000120  d9 d7 82 69 00 02 00 00  ab 11 1e c8 96 66 73 cb  |...i.........fs.|
00000130  b0 bb 37 0c 01 03 06 0c  0f 1a 21 2a 72 77 78 79  |..7.......!*rwxy|
00000140  39 02 05 c0 50 00 0c 05  67 6f 65 6d 75 ff        |9...P...goemu.|
`,
	`00000000  ff ff ff ff ff ff 36 8c  76 8e 5e 96 08 00 45 c0  |......6.v.^...E.|
00000010  01 4a 00 00 00 00 40 11  78 e4 00 00 00 00 ff ff  |.J....@.x.......|
00000020  ff ff 00 44 00 43 01 36  56 ad 01 01 06 00 c8 33  |...D.C.6V......3|
00000030  c1 eb 00 01 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000040  00 00 00 00 00 00 36 8c  76 8e 5e 96 00 00 00 00  |......6.v.^.....|
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
00000100  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000110  00 00 00 00 00 00 63 82  53 63 35 01 03 3d 13 ff  |......c.Sc5..=..|
00000120  d9 d7 82 69 00 02 00 00  ab 11 1e c8 96 66 73 cb  |...i.........fs.|
00000130  b0 bb 37 0c 01 03 06 0c  0f 1a 21 2a 72 77 78 79  |..7.......!*rwxy|
00000140  39 02 05 c0 36 04 c0 a8  2a fe 32 04 c0 a8 2a 02  |9...6...*.2...*.|
00000150  0c 05 67 6f 65 6d 75 ff                           |..goemu.|
`,
}

func TestDHCPDecode(t *testing.T) {
	server := NewServer("", net.IP([]byte{
		192, 168, 42, 254,
	}))
	server.DomainName = "goemu.markkurossi.com"
	server.DNS = append(server.DNS, net.IP([]byte{
		8, 8, 8, 8,
	}))
	server.DNS = append(server.DNS, net.IP([]byte{
		1, 1, 1, 1,
	}))
	guestMAC := network.MAC([]byte{
		0x36, 0x8c, 0x76, 0x8e, 0x5e, 0x96,
	})
	hostMAC := network.MAC([]byte{
		0x7e, 0xf1, 0x23, 0x40, 0x3f, 0xe6,
	})
	guestIP := net.IP([]byte{192, 168, 42, 1})
	hostIP := net.IP([]byte{192, 168, 42, 254})

	server.AddClient(guestMAC, &ClientInfo{
		IP:       guestIP,
		Hostname: "freebsd",
	})

	for idx, packet := range dhcpPackets {
		data, err := hexdump.Parse([]byte(packet))
		if err != nil {
			t.Errorf("DHCP-%v: failed to parse packet: %v", idx, err)
			continue
		}

		// Skip Ethernet frame.
		data = data[14:]

		fmt.Printf("IP:\n%s", hex.Dump(data[:20]))
		if !network.VerifyChecksum(data) {
			t.Errorf("IP packet checksum verification failed")
		}
		fmt.Printf("computed checksum: %v\n", network.ComputeChecksum(data))
		fmt.Printf("UDP:\n%s", hex.Dump(data[20:28]))
		if !network.VerifyUDPChecksum(data) {
			t.Errorf("UDP checksum verification failed")
		}
		fmt.Printf("computed UDP checksum: %v\n",
			network.ComputeUDPChecksum(data))
		if !network.VerifyUDPChecksum(data) {
			t.Errorf("computed UDP checksum verification failed")
		}

		req, err := Decode(data[28:])
		if err != nil {
			t.Errorf("DecodeDHCP-%v: %v", idx, err)
			continue
		}
		fmt.Printf("DHCP:%v:0: %v\n", idx, req)
		if req.Cookie != OptionsCookie {
			t.Errorf("DHCP-%v: invalid cookie %x, expected %x",
				idx, req.Cookie, OptionsCookie)
		}

		resp, err := server.Request(req)
		if err != nil {
			t.Fatalf("DHCP-%v: failed to respond to %v: %v",
				idx, req.MsgType, err)
		}

		fmt.Printf("DHCP:%v:1: %v\n", idx, resp)

		rdata := resp.Encode()
		fmt.Printf("Response:\n%s", hex.Dump(rdata))
		dr, err := Decode(rdata)
		if err != nil {
			t.Fatalf("DHCP-%v: failed to decode encoded response: %v", idx, err)
		}
		fmt.Printf("dhcp:%v:2: %v\n", idx, dr)

		// Construct an offer packet.
		data = make([]byte, 12+14+20+8+len(rdata))

		network.MakeEthernet(data[12:], guestMAC, hostMAC, network.EthernetIPv4)

		// IP.
		ip := 12 + 14
		data[ip+0] = 0x45
		data[ip+1] = 0x10
		network.BO.PutUint16(data[ip+2:], uint16(20+8+len(rdata)))
		data[ip+8] = 0x80 // TTL
		data[ip+9] = 0x11 // UDP
		copy(data[ip+12:], hostIP)
		copy(data[ip+16:], network.BroadcastIP)
		network.ComputeChecksum(data[ip:])
		fmt.Printf("IP:\n%s", hex.Dump(data[ip:ip+20]))

		// UDP.
		network.BO.PutUint16(data[ip+20:], 67)
		network.BO.PutUint16(data[ip+22:], 68)
		network.BO.PutUint16(data[ip+24:], uint16(8+len(rdata)))
		copy(data[ip+28:], rdata)
		network.ComputeUDPChecksum(data[ip:])
		fmt.Printf("UDP:\n%s", hex.Dump(data[ip+20:ip+28]))
		fmt.Printf("DHCP:\n%s", hex.Dump(rdata))
		fmt.Printf("UDP payload:\n%s", hex.Dump(data[ip+28:]))

		fmt.Printf("Full DHCPOFFER:\n%s", hex.Dump(data[ip:]))
	}
}
