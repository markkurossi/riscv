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

type MsgType uint8

const (
	DHCPDISCOVER MsgType = iota + 1
	DHCPOFFER
	DHCPREQUEST
	DHCPDECLINE
	DHCPACK
	DHCPNAK
	DHCPRELEASE
	DHCPINFORM
)

var msgTypes = map[MsgType]string{
	DHCPDISCOVER: "DHCPDISCOVER",
	DHCPOFFER:    "DHCPOFFER",
	DHCPREQUEST:  "DHCPREQUEST",
	DHCPDECLINE:  "DHCPDECLINE",
	DHCPACK:      "DHCPACK",
	DHCPNAK:      "DHCPNAK",
	DHCPRELEASE:  "DHCPRELEASE",
	DHCPINFORM:   "DHCPINFORM",
}

func (t MsgType) String() string {
	name, ok := msgTypes[t]
	if ok {
		return name
	}
	return fmt.Sprintf("{MsgType %d}", int(t))
}

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
	TagStaticRoute                OptionTag = 33
	TagAddressRequest             OptionTag = 50
	TagAddressTime                OptionTag = 51
	TagDHCPMsgType                OptionTag = 53
	TagDHCPServerID               OptionTag = 54
	TagParameterList              OptionTag = 55
	TagMessage                    OptionTag = 56
	TagDHCPMaxMsgSize             OptionTag = 57
	TagClientID                   OptionTag = 61
	TagAutoConfig                 OptionTag = 116
	TagDomainSearch               OptionTag = 119
	TagClasslessStaticRouteOption OptionTag = 121
	TagForcerenewNonceCapable     OptionTag = 145
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
	TagStaticRoute:      {'I', "Static Route"},
	TagAddressRequest:   {'i', "Address Request"},
	TagAddressTime:      {'d', "Address Time"},
	TagDHCPMsgType:      {'x', "DHCP Msg Type"},
	TagDHCPServerID:     {'i', "DHCP Server Id"},
	TagParameterList:    {'P', "Parameter List"},
	TagMessage:          {'s', "Message"},
	TagDHCPMaxMsgSize:   {'d', "DHCP Max Msg Size"},
	TagClientID:         {'x', "Client Id"},
	TagEnd:              {'0', "End"},

	// RFC 2563 - DHCP Auto-Configuration Option
	TagAutoConfig: {'d', "Auto-Config"},

	// RFC 3397 - DHCP Domain Search Option
	TagDomainSearch: {'x', "Domain Search"},

	// RFC 3442 - Classless Static Route Option for DHCPv4
	TagClasslessStaticRouteOption: {'x', "Classless Static Route Option"},

	// RFC 6704 - Forcerenew Nonce
	TagForcerenewNonceCapable: {'d', "FORCERENEW_NONCE_CAPABLE"},
}
