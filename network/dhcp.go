//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package network

import (
	"fmt"
	"io"
	"net"
	"strings"
)

const (
	BOOTREQUEST       uint8  = 1
	BOOTREPLY         uint8  = 2
	DHCPOptionsCookie uint32 = 0x63825363
)

type DHCP struct {
	Op      uint8
	HType   uint8
	HLen    uint8
	Hops    uint8
	XID     uint32
	Secs    uint16
	Flags   uint16
	CIAddr  net.IP
	YIAddr  net.IP
	SIAddr  net.IP
	GIAddr  net.IP
	CHAddr  HAddr
	SName   NullString
	File    NullString
	Cookie  uint32
	Options []Option
}

func (dhcp *DHCP) AddOption(opt Option) {
	dhcp.Options = append(dhcp.Options, opt)
}

func NewNullString(v string) NullString {
	bytes := []byte(v)
	if len(bytes) > 63 {
		bytes = bytes[:63]
	}
	bytes = append(bytes, 0)
	return NullString(bytes)
}

type NullString []byte

func (ns NullString) String() string {
	var i int
	for i = 0; i < len(ns) && ns[i] != 0; i++ {
	}
	return string(ns[:i])
}

type HAddr []byte

func (haddr HAddr) String() string {
	var parts []string

	for _, b := range haddr {
		parts = append(parts, fmt.Sprintf("%02x", b))
	}
	return strings.Join(parts, ":")
}

func DecodeDHCP(data []byte) (*DHCP, error) {
	d := &decoder{
		data: data,
	}

	dhcp := &DHCP{
		Op:     d.Uint8(),
		HType:  d.Uint8(),
		HLen:   d.Uint8(),
		Hops:   d.Uint8(),
		XID:    d.Uint32(),
		Secs:   d.Uint16(),
		Flags:  d.Uint16(),
		CIAddr: d.IP(),
		YIAddr: d.IP(),
		SIAddr: d.IP(),
		GIAddr: d.IP(),
		CHAddr: d.HAddr(16),
		SName:  d.String(64),
		File:   d.String(128),
		Cookie: d.Uint32(),
	}
	if d.err != nil {
		return nil, d.err
	}

	// Parse options.
	for len(d.data) > 0 && d.err == nil {
		option := Option{
			Tag: d.Uint8(),
		}
		switch option.Tag {
		case 0, 0xff:

		default:
			l := d.Uint8()
			option.Data = d.Data(int(l))
		}
		dhcp.Options = append(dhcp.Options, option)
	}

	return dhcp, nil
}

type decoder struct {
	data []byte
	err  error
}

func (d *decoder) Uint8() uint8 {
	if d.err != nil {
		return 0
	}
	if len(d.data) == 0 {
		d.err = io.EOF
		return 0
	}
	v := d.data[0]
	d.data = d.data[1:]

	return v
}

func (d *decoder) Uint16() uint16 {
	if d.err != nil {
		return 0
	}
	if len(d.data) < 2 {
		d.err = io.EOF
		return 0
	}
	v := BO.Uint16(d.data)
	d.data = d.data[2:]

	return v
}

func (d *decoder) Uint32() uint32 {
	if d.err != nil {
		return 0
	}
	if len(d.data) < 4 {
		d.err = io.EOF
		return 0
	}
	v := BO.Uint32(d.data)
	d.data = d.data[4:]

	return v
}

func (d *decoder) IP() net.IP {
	if d.err != nil {
		return ZeroIP
	}
	if len(d.data) < 4 {
		d.err = io.EOF
		return ZeroIP
	}
	v := net.IP(d.data[:4])
	d.data = d.data[4:]

	return v
}

func (d *decoder) Data(size int) []byte {
	if d.err != nil {
		return nil
	}
	if len(d.data) < size {
		d.err = io.EOF
		return nil
	}
	v := d.data[:size]
	d.data = d.data[size:]

	return v
}

func (d *decoder) HAddr(size int) HAddr {
	return HAddr(d.Data(size))
}

func (d *decoder) String(size int) NullString {
	return NullString(d.Data(size))
}
