//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package dhcp

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/markkurossi/riscv/network"
)

type Server struct {
	Hostname   NullString
	Netmask    net.IP
	Broadcast  net.IP
	GW         net.IP
	DomainName string
	DNS        []net.IP
	MTU        uint16

	Clients map[network.MAC]*ClientInfo
}

type ClientInfo struct {
	IP       net.IP
	Hostname string
}

func (ci ClientInfo) DomainName() string {
	idx := strings.IndexByte(ci.Hostname, '.')
	if idx >= 0 {
		return ci.Hostname[idx+1:]
	}
	return ""
}

func NewServer(hostname string, gw net.IP) *Server {
	return &Server{
		Hostname: NewNullString(hostname),
		GW:       gw,
		Clients:  make(map[network.MAC]*ClientInfo),
	}
}

func (server *Server) AddClient(mac network.MAC, client *ClientInfo) {
	server.Clients[mac] = client
}

// Discover creates a DHCPOFFER message for the DHCPDISCOVER message req.
func (server *Server) Discover(req *DHCP) (*DHCP, error) {
	if req.Op != BOOTREQUEST {
		return nil, fmt.Errorf("invalid DHCPDISCOVER op %v", req.Op)
	}
	if req.MsgType != DHCPDISCOVER {
		return nil, fmt.Errorf("invalid DHCPDISCOVER msg type %v", req.MsgType)
	}
	clientMAC := network.MAC(req.CHAddr[0:6])
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
		CIAddr: network.ZeroIP,
		YIAddr: client.IP,
		SIAddr: server.GW,
		GIAddr: req.GIAddr,
		CHAddr: req.CHAddr,
		SName:  server.Hostname,
		Cookie: OptionsCookie,
	}

	// Add mandatory server options.

	// DHCP Msg Type.
	resp.AddOption(Option{
		Tag:  TagDHCPMsgType,
		Data: []byte{DHCPOFFER},
	})
	// IP Address Lease Time.
	resp.AddOption(Option{
		Tag:  TagAddressTime,
		Data: Uint32Data(86400),
	})
	// Server Identifier.
	resp.AddOption(Option{
		Tag:  TagDHCPServerID,
		Data: []byte(server.GW),
	})

	// Process request options.
	for idx, opt := range req.Options {
		switch opt.Tag {
		case TagPad, TagEnd:

		case TagParameterList:
			for pi, p := range opt.Data {
				switch OptionTag(p) {
				case TagSubnetMask:
					resp.AddOption(Option{
						Tag:  TagSubnetMask,
						Data: []byte(server.netmask()),
					})

				case TagBroadcastAddress:
					resp.AddOption(Option{
						Tag:  TagBroadcastAddress,
						Data: []byte(server.broadcast()),
					})

				case TagRouter:
					resp.AddOption(Option{
						Tag:  TagRouter,
						Data: []byte(server.GW),
					})

				case TagDomainServer:
					if len(server.DNS) > 0 {
						opt := Option{
							Tag: TagDomainServer,
						}
						for _, dns := range server.DNS {
							opt.Append(dns)
						}
						resp.AddOption(opt)
					}

				case TagHostname:
					if len(client.Hostname) > 0 {
						resp.AddOption(Option{
							Tag:  TagHostname,
							Data: []byte(client.Hostname),
						})
					}

				case TagDomainName:
					name := client.DomainName()
					if len(name) == 0 {
						name = server.domainName()
					}
					if len(name) > 0 {
						resp.AddOption(Option{
							Tag:  TagDomainName,
							Data: []byte(name),
						})
					}

				default:
					log.Printf("ignoring parameter %v: ", pi)
					pt, ok := options[OptionTag(p)]
					if ok {
						log.Printf("%v\n", pt.Name)
					} else {
						log.Printf("%v\n", pt)
					}
				}
			}
		default:
			log.Printf("option %v: %v\n", idx, opt)
		}
	}

	// End.
	resp.AddOption(Option{
		Tag: TagEnd,
	})

	return resp, nil
}

// Request creates a DHCPACK message for the DHCPREQUEST message req.
func (server *Server) Request(req *DHCP) (*DHCP, error) {
	if req.Op != BOOTREQUEST {
		return nil, fmt.Errorf("invalid DHCPREQUEST op %v", req.Op)
	}
	if req.MsgType != DHCPREQUEST {
		return nil, fmt.Errorf("invalid DHCPREQUEST msg type %v", req.MsgType)
	}
	clientMAC := network.MAC(req.CHAddr[0:6])
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
		CIAddr: req.CIAddr,
		YIAddr: client.IP,
		SIAddr: server.GW,
		GIAddr: req.GIAddr,
		CHAddr: req.CHAddr,
		SName:  server.Hostname,
		Cookie: OptionsCookie,
	}

	// Add mandatory server options.

	// DHCP Msg Type.
	resp.AddOption(Option{
		Tag:  TagDHCPMsgType,
		Data: []byte{DHCPACK},
	})
	// IP Address Lease Time.
	resp.AddOption(Option{
		Tag:  TagAddressTime,
		Data: Uint32Data(86400),
	})
	// Server Identifier.
	resp.AddOption(Option{
		Tag:  TagDHCPServerID,
		Data: []byte(server.GW),
	})

	// Process request options.
	for idx, opt := range req.Options {
		switch opt.Tag {
		case TagPad, TagEnd:

		case TagParameterList:
			for pi, p := range opt.Data {
				switch OptionTag(p) {
				case TagSubnetMask:
					resp.AddOption(Option{
						Tag:  TagSubnetMask,
						Data: []byte(server.netmask()),
					})

				case TagBroadcastAddress:
					resp.AddOption(Option{
						Tag:  TagBroadcastAddress,
						Data: []byte(server.broadcast()),
					})

				case TagRouter:
					resp.AddOption(Option{
						Tag:  TagRouter,
						Data: []byte(server.GW),
					})

				case TagDomainServer:
					if len(server.DNS) > 0 {
						opt := Option{
							Tag: TagDomainServer,
						}
						for _, dns := range server.DNS {
							opt.Append(dns)
						}
						resp.AddOption(opt)
					}

				case TagHostname:
					if len(client.Hostname) > 0 {
						resp.AddOption(Option{
							Tag:  TagHostname,
							Data: []byte(client.Hostname),
						})
					}

				case TagDomainName:
					name := client.DomainName()
					if len(name) == 0 {
						name = server.domainName()
					}
					if len(name) > 0 {
						resp.AddOption(Option{
							Tag:  TagDomainName,
							Data: []byte(name),
						})
					}

				default:
					log.Printf("ignoring parameter %v: ", pi)
					pt, ok := options[OptionTag(p)]
					if ok {
						log.Printf("%v\n", pt.Name)
					} else {
						log.Printf("%v\n", pt)
					}
				}
			}
		default:
			log.Printf("option %v: %v\n", idx, opt)
		}
	}

	// End.
	resp.AddOption(Option{
		Tag: TagEnd,
	})

	return resp, nil
}

func (server *Server) netmask() net.IP {
	if !network.IsZeroIP(server.Netmask) {
		return server.Netmask
	}
	return net.IP(server.GW.DefaultMask())
}

func (server *Server) broadcast() net.IP {
	if !network.IsZeroIP(server.Broadcast) {
		return server.Broadcast
	}
	if server.GW.IsUnspecified() {
		return server.GW
	}
	bcast := make([]byte, len(server.GW))
	copy(bcast, server.GW)
	bcast[len(bcast)-1] = 0xff

	return net.IP(bcast)
}

func (server *Server) domainName() string {
	if len(server.DomainName) > 0 {
		return server.DomainName
	}
	idx := bytes.IndexByte(server.Hostname, '.')
	if idx >= 0 {
		return server.Hostname[idx+1:].String()
	}
	return ""
}

func Uint32Data(v uint32) []byte {
	buf := make([]byte, 4)
	network.BO.PutUint32(buf, v)
	return buf
}
