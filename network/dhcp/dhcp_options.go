//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package dhcp

import (
	"fmt"
	"net"
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

type OptionTag uint8

const (
	TagPad                        OptionTag = 0
	TagSubnetMask                 OptionTag = 1
	TagTimeOffset                 OptionTag = 2
	TagRouter                     OptionTag = 3
	TagDomainServer               OptionTag = 6
	TagHostname                   OptionTag = 12
	TagDomainName                 OptionTag = 15
	TagMTUInterface               OptionTag = 26
	TagBroadcastAddress           OptionTag = 28
	TagAddressRequest             OptionTag = 50
	TagAddressTime                OptionTag = 51
	TagDHCPMsgType                OptionTag = 53
	TagDHCPServerID               OptionTag = 54
	TagParameterList              OptionTag = 55
	TagMessage                    OptionTag = 56
	TagClientID                   OptionTag = 61
	TagDomainSearch               OptionTag = 119
	TagClasslessStaticRouteOption OptionTag = 121
	TagEnd                        OptionTag = 255
)

func (tag OptionTag) String() string {
	ot, ok := options[tag]
	if ok {
		return ot.Name
	}
	return fmt.Sprintf("%v", int(tag))
}

type Option struct {
	Tag  OptionTag
	Data []byte
}

func (opt Option) Len() int {
	switch opt.Tag {
	case TagPad, TagEnd:
		return 1
	default:
		return 1 + 1 + len(opt.Data)
	}
}

func (opt *Option) Append(data []byte) {
	opt.Data = append(opt.Data, data...)
}

func (opt Option) Uint8() (uint8, error) {
	if len(opt.Data) != 1 {
		return 0, fmt.Errorf("Uint8: len=%v", len(opt.Data))
	}
	return opt.Data[0], nil
}

func (opt Option) String() string {
	ot, ok := options[opt.Tag]
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
	case 'd':
		var v uint64
		for _, b := range opt.Data {
			v <<= 8
			v |= uint64(b)
		}
		return fmt.Sprintf("%v=%v", ot.Name, v)

	case 'i':
		return fmt.Sprintf("%v=%v", ot.Name, net.IP(opt.Data))

	case 'I':
		var addrs []net.IP
		data := opt.Data
		for len(data) > 0 {
			l := len(opt.Data)
			if l > 4 {
				l = 4
			}
			addrs = append(addrs, net.IP(data[0:l]))
			data = data[l:]
		}
		result := fmt.Sprintf("%v=", ot.Name)
		for idx, ip := range addrs {
			if idx > 0 {
				result += ","
			}
			result += ip.String()
		}
		return result

	case 'P':
		var params []string
		for _, p := range opt.Data {
			pt, ok := options[OptionTag(p)]
			if ok {
				params = append(params, pt.Name)
			} else {
				params = append(params, fmt.Sprintf("%v", p))
			}
		}
		return fmt.Sprintf("%v=%v", ot.Name, strings.Join(params, ","))

	case 's':
		return fmt.Sprintf("%v=%s", ot.Name, string(opt.Data))

	default:
		return fmt.Sprintf("%v=%x", ot.Name, opt.Data)
	}
}

type OptType struct {
	Type rune
	Name string
}

var options = map[OptionTag]OptType{
	// RFC 2132 - DHCP Options and BOOTP Vendor Extensions
	TagPad:              {'0', "Pad"},
	TagSubnetMask:       {'i', "Subnet Mask"},
	TagTimeOffset:       {'d', "Time Offset"},
	TagRouter:           {'I', "Router"},
	TagDomainServer:     {'I', "Domain Server"},
	TagHostname:         {'s', "Hostname"},
	TagDomainName:       {'s', "Domain Name"},
	TagMTUInterface:     {'d', "MTU Interface"},
	TagBroadcastAddress: {'i', "Broadcast Address"},
	TagAddressRequest:   {'i', "Address Request"},
	TagAddressTime:      {'d', "Address Time"},
	TagDHCPMsgType:      {'x', "DHCP Msg Type"},
	TagDHCPServerID:     {'i', "DHCP Server Id"},
	TagParameterList:    {'P', "Parameter List"},
	TagMessage:          {'s', "Message"},
	TagClientID:         {'x', "Client Id"},
	TagEnd:              {'0', "End"},

	// RFC 3397 - DHCP Domain Search Option
	TagDomainSearch: {'x', "Domain Search"},

	// RFC 3442 - Classless Static Route Option for DHCPv4
	TagClasslessStaticRouteOption: {'x', "Classless Static Route Option"},
}
