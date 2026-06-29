//
// tunnel_darwin.go
//
// Copyright (c) 2019-2026 Markku Rossi
//
// All rights reserved.
//

package tun

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"text/template"

	"golang.org/x/sys/unix"
)

// Standard Darwin AF_INET / AF_INET6 values
const (
	afInet  uint32 = 2
	afInet6 uint32 = 30
)

var setCommands = []string{
	"/sbin/ifconfig {{.Iface}} {{.LocalIP}} {{.RemoteIP}} up",
	//"ifconfig {{.Iface}} inet6 {{.LocalIP6}} {{.RemoteIP6}} prefixlen 128 up",

	// Add route to the VPN server via current default GW
	//"route add {{.ServerIP}} {{.GatewayIP}}",

	// Default route via VPN
	//"route add 0/1 {{.RemoteIP}}",
	//"route add 128/1 {{.RemoteIP}}",

	//"route add -inet6 -blackhole 0000::/1 {{.RemoteIP6}}",
	//"route add -inet6 -blackhole 8000::/1 {{.RemoteIP6}}",
}

var unsetCommands = []string{
	"route delete {{.ServerIP}} {{.GatewayIP}}",
}

type config struct {
	Config
	Iface string
}

// Configure configures the virtual interface according to the
// configuration parameters.
func (t *Tunnel) Configure(cfg Config) error {
	for _, command := range setCommands {
		tmpl := template.Must(template.New("set").Parse(command))

		builder := new(strings.Builder)
		err := tmpl.Execute(builder, config{
			Iface:  t.Name,
			Config: cfg,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Command: %s\n", builder.String())

		args := strings.Split(builder.String(), " ")

		cmd := exec.Command(args[0], args[1:]...)
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	return nil
}

// Read reads a raw IP packet from the utun interface.
func (t *Tunnel) Read(p []byte) (int, error) {
	// Allocate a temporary buffer on the stack large enough to hold
	// the 4-byte family header + payload.
	buf := make([]byte, len(p)+4)

	n, err := unix.Read(int(t.fd), buf)
	if err != nil {
		return 0, err
	}
	if n <= 4 {
		return 0, nil
	}

	// Strip the 4-byte family header and return the raw packet data
	copy(p, buf[4:n])
	return n - 4, nil
}

// Write writes a raw IP packet out to the utun interface.
func (t *Tunnel) Write(p []byte) (int, error) {
	if len(p) < 20 {
		return 0, nil
	}

	var family uint32
	ipVersion := p[0] >> 4

	switch ipVersion {
	case 4:
		family = afInet
	case 6:
		family = afInet6
	default:
		return 0, errors.New("invalid IP version")
	}

	// Allocate space to prepend the 4-byte family header natively in Go
	buf := make([]byte, len(p)+4)
	binary.BigEndian.PutUint32(buf[0:4], family)
	copy(buf[4:], p)

	n, err := unix.Write(int(t.fd), buf)
	if err != nil {
		return 0, err
	}
	if n <= 4 {
		return 0, nil
	}

	return n - 4, nil
}
