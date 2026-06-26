//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package network

import (
	"fmt"
	"strings"
)

const (
	DHCPDISCOVER uint8 = 1
	DHCPOFFER    uint8 = 2
	DHCPREQUEST  uint8 = 3
	DHCPDECLINE  uint8 = 4
	DHCPACK      uint8 = 5
	DHCPNAK      uint8 = 6
	DHCPRELEASE  uint8 = 7
	DHCPINFORM   uint8 = 8
)

type Option struct {
	Tag  uint8
	Data []byte
}

func (opt Option) Uint8() (uint8, error) {
	if len(opt.Data) != 1 {
		return 0, fmt.Errorf("Uint8: len=%v", len(opt.Data))
	}
	return opt.Data[0], nil
}

func (opt Option) String() string {
	ot, ok := dhcpOptions[opt.Tag]
	if !ok {
		ot = OptType{
			Type: 'x',
			Name: fmt.Sprintf("%v", opt.Tag),
		}
	}
	if len(opt.Data) == 0 {
		return ot.Name
	}
	switch ot.Type {
	case 's':
		return fmt.Sprintf("%v=%s", ot.Name, string(opt.Data))
	case 'P':
		var params []string
		for _, p := range opt.Data {
			pt, ok := dhcpOptions[p]
			if ok {
				params = append(params, pt.Name)
			} else {
				params = append(params, fmt.Sprintf("%v", p))
			}
		}
		return fmt.Sprintf("%v=%v", ot.Name, strings.Join(params, ","))
	default:
		return fmt.Sprintf("%v=%x", ot.Name, opt.Data)
	}
}

type OptType struct {
	Type rune
	Name string
}

var dhcpOptions = map[uint8]OptType{
	// RFC 2132 - DHCP Options and BOOTP Vendor Extensions
	0:   {'0', "Pad"},
	1:   {'i', "Subnet Mask"},
	2:   {'d', "Time Offset"},
	3:   {'I', "Router"},
	6:   {'I', "Domain Server"},
	12:  {'s', "Hostname"},
	15:  {'s', "Domain Name"},
	26:  {'d', "MTU Interface"},
	28:  {'i', "Broadcast Address"},
	51:  {'d', "Address Time"},
	53:  {'x', "DHCP Msg Type"},
	54:  {'i', "DHCP Server Id"},
	55:  {'P', "Parameter List"},
	56:  {'s', "Message"},
	61:  {'x', "Client Id"},
	255: {'0', "End"},

	// RFC 3397 - DHCP Domain Search Option
	119: {'x', "Domain Search"},

	// RFC 3442 - Classless Static Route Option for DHCPv4
	121: {'x', "Classless Static Route Option"},
}
