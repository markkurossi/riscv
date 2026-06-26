//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

// Package dhcp implements the Dynamic Host Configuration Protocol
// (DHCP) defined by RFC 2131.
package dhcp

import (
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/markkurossi/riscv/network"
)

const (
	BOOTREQUEST   uint8  = 1
	BOOTREPLY     uint8  = 2
	HdrLen               = 7*4 + 16 + 64 + 128 + 4
	OptionsCookie uint32 = 0x63825363
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

func (dhcp *DHCP) Encode() []byte {
	l := HdrLen

	// Count options length.
	for _, opt := range dhcp.Options {
		l += opt.Len()
	}
	l = (l + 3) / 4 * 4
	fmt.Printf("len=%v\n", l)

	buf := make([]byte, l)
	buf[0] = dhcp.Op
	buf[1] = dhcp.HType
	buf[2] = dhcp.HLen
	buf[3] = dhcp.Hops
	network.BO.PutUint32(buf[4:], dhcp.XID)
	network.BO.PutUint16(buf[8:], dhcp.Secs)
	network.BO.PutUint16(buf[10:], dhcp.Flags)
	copy(buf[12:], dhcp.CIAddr)
	copy(buf[16:], dhcp.YIAddr)
	copy(buf[20:], dhcp.SIAddr)
	copy(buf[24:], dhcp.GIAddr)
	copy(buf[28:], dhcp.CHAddr)
	copy(buf[44:], []byte(dhcp.SName))
	copy(buf[108:], []byte(dhcp.File))
	network.BO.PutUint32(buf[236:], OptionsCookie)

	// Encode options.
	ofs := 240
	for _, opt := range dhcp.Options {
		buf[ofs] = byte(opt.Tag)
		ofs++
		switch opt.Tag {
		case TagPad, TagEnd:
		default:
			buf[ofs] = byte(len(opt.Data))
			ofs++
			ofs += copy(buf[ofs:], opt.Data)
		}
	}

	return buf
}

func Decode(data []byte) (*DHCP, error) {
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
			Tag: OptionTag(d.Uint8()),
		}
		switch option.Tag {
		case TagPad, TagEnd:

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
	v := network.BO.Uint16(d.data)
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
	v := network.BO.Uint32(d.data)
	d.data = d.data[4:]

	return v
}

func (d *decoder) IP() net.IP {
	if d.err != nil {
		return network.ZeroIP
	}
	if len(d.data) < 4 {
		d.err = io.EOF
		return network.ZeroIP
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
