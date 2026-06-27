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
	`00000000  45 10 01 48 00 00 00 00  80 11 39 96 00 00 00 00  |E..H......9.....|
00000010  ff ff ff ff 00 44 00 43  01 34 c7 87 01 01 06 00  |.....D.C.4......|
00000020  c7 ca fe cb 00 00 00 00  00 00 00 00 00 00 00 00  |................|
00000030  00 00 00 00 00 00 00 00  fe 53 c5 87 31 93 00 00  |.............j..|
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
00000100  00 00 00 00 00 00 00 00  63 82 53 63 35 01 03 36  |........c.Sc5..6|
00000110  10 00 00 00 00 00 00 00  00 00 00 ff ff c0 a8 2a  |...............*|
00000120  fe 32 04 00 00 00 00 3d  07 01 0a e2 84 e1 11 6a  |.2.....=.......j|
00000130  0c 07 66 72 65 65 62 73  64 37 0a 01 1c 02 79 03  |..freebsd7....y.|
00000140  0f 06 0c 77 1a ff 00 00                           |...w....|
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
		0xfe, 0x53, 0xc5, 0x87, 0x31, 0x93,
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

		var resp *DHCP
		var rdata []byte

		switch req.MsgType {
		case DHCPDISCOVER:
			resp, err = server.Discover(req)
			if err != nil {
				t.Fatalf("DHCP-%v: failed to respond to DHCPDISCOVER: %v",
					idx, err)
			}

		case DHCPREQUEST:
			resp, err = server.Request(req)
			if err != nil {
				t.Fatalf("DHCP-%v: failed to respond to DHCPREQUEST: %v",
					idx, err)
			}

		default:
			t.Fatalf("DHCP-%v: invalid message type: %v", idx, req.MsgType)
		}
		fmt.Printf("DHCP:%v:1: %v\n", idx, resp)

		rdata = resp.Encode()
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
