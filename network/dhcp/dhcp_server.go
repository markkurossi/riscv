//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package network

import (
	"fmt"
	"net"
)

type DHCPServer struct {
	Hostname   NullString
	Netmask    net.IP
	Broadcast  net.IP
	GW         net.IP
	DomainName string
	DNS        []net.IP
	MTU        uint16

	Clients map[MAC]*ClientInfo
}

type ClientInfo struct {
	IP      net.IP
	Hotname string
}

func NewDHCPServer(hostname string, gw net.IP) *DHCPServer {
	return &DHCPServer{
		Hostname: NewNullString(hostname),
		GW:       gw,
		Clients:  make(map[MAC]*ClientInfo),
	}
}

func (server *DHCPServer) AddClient(mac MAC, client *ClientInfo) {
	server.Clients[mac] = client
}

// Offer creates a DHCPOFFER message for the DHCPDISCOVER message req.
func (server *DHCPServer) Offer(req *DHCP) (*DHCP, error) {
	if req.Op != BOOTREQUEST {
		return nil, fmt.Errorf("invalid DHCPDISCOVER op %v", req.Op)
	}
	clientMAC := MAC(req.CHAddr[0:6])
	client, ok := server.Clients[clientMAC]
	if !ok {
		return nil, fmt.Errorf("unknown client %v", clientMAC)
	}

	resp := &DHCP{
		Op:     BOOTREPLY,
		HType:  req.HType,
		HLen:   6,
		XID:    req.XID,
		Flags:  req.Flags,
		CIAddr: ZeroIP,
		YIAddr: client.IP,
		SIAddr: server.GW,
		GIAddr: req.GIAddr,
		CHAddr: req.CHAddr,
		SName:  server.Hostname,
		Cookie: DHCPOptionsCookie,
	}

	// Add mandatory server options.

	// IP Address Lease Time.
	resp.AddOption(Option{
		Tag:  51,
		Data: Uint32Data(0xffffffff),
	})
	// Server Identifier.
	resp.AddOption(Option{
		Tag:  54,
		Data: []byte(server.GW),
	})

	// Process request options.

	var reqMsgType uint8
	var err error

	fmt.Printf("Options:\n")
	for idx, opt := range req.Options {
		fmt.Printf(" - %v\t%v\n", idx, opt)
		switch opt.Tag {
		case 0, 0xff:

		case 53:
			reqMsgType, err = opt.Uint8()
			if err != nil {
				return nil, fmt.Errorf("invalid option %v: %w", opt, err)
			}

		case 55: // Parameter List
			for pi, p := range opt.Data {
				switch p {
				default:
					fmt.Printf("Parameter list %v: ", pi)
					pt, ok := dhcpOptions[p]
					if ok {
						fmt.Printf("%v\n", pt.Name)
					} else {
						fmt.Printf("%v\n", pt)
					}
				}
			}
		}
	}

	// End.
	resp.AddOption(Option{
		Tag: 0xff,
	})

	if reqMsgType != DHCPDISCOVER {
		return nil, fmt.Errorf("invalid DHCPDISCOVER msg type %v", reqMsgType)
	}

	return resp, nil
}

func Uint32Data(v uint32) []byte {
	buf := make([]byte, 4)
	BO.PutUint32(buf, v)
	return buf
}
