//
// tunnel.go
//
// Copyright (c) 2019-2026 Markku Rossi
//
// All rights reserved.
//

// Package tun implements virtual tunnels.
package tun

// #include "tunnel.h"
import "C"

import (
	"errors"
	"unsafe"
)

const (
	// DefaultClientIP defines the default tunnel local IP address.
	DefaultClientIP = "192.168.192.1"
	// DefaultServerIP defines the default tunnel remote IP address.
	DefaultServerIP = "192.168.192.254"
)

// EXT_IP is the VPN server address
// EXT_GW_IP is the default gw

// Config defines the tunnel configuration information.
type Config struct {
	LocalIP   string
	RemoteIP  string
	LocalIP6  string
	RemoteIP6 string
	ServerIP  string
	GatewayIP string
}

// Tunnel implements the virtual interface.
type Tunnel struct {
	fd   C.int
	Name string
}

// Create creates a new virtual interface.
func Create() (*Tunnel, error) {
	var namePtr *C.char
	var errno C.int

	ret := C.tun_create(&namePtr, &errno)
	if ret < 0 {
		return nil, errors.New(C.GoString(C.strerror(errno)))
	}

	name := C.GoString(namePtr)
	C.free(unsafe.Pointer(namePtr))

	return &Tunnel{
		fd:   ret,
		Name: name,
	}, nil
}

func (t *Tunnel) String() string {
	return t.Name
}
