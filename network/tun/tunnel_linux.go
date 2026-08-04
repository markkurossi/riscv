//
// tunnel_linux.go
//
// Copyright (c) 2019-2026 Markku Rossi
//
// All rights reserved.
//

package tun

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"text/template"

	"golang.org/x/sys/unix"
)

var setCommands = []string{
	"ip addr add {{.LocalIP}} peer {{.RemoteIP}} dev {{.Iface}}",
	// "ip -6 addr add $LOCAL_TUN_IP6 peer $REMOTE_TUN_IP6/96 dev $IF_NAME",
	"ip link set dev {{.Iface}} up",
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

// Read reads a raw IP packet from the Linux tun interface.
func (t *Tunnel) Read(p []byte) (int, error) {
	for {
		n, err := unix.Read(int(t.fd), p)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return 0, err
		}
		return n, nil
	}
}

// Write writes a raw IP packet out to the Linux tun interface.
func (t *Tunnel) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	var writtenTotal int
	for writtenTotal < len(p) {
		n, err := unix.Write(int(t.fd), p[writtenTotal:])
		if err == nil {
			writtenTotal += n
			continue
		}

		// Handle retry logic if signals interrupt the write step
		if err == unix.EINTR {
			continue
		}

		// Handle short writes or blocking states if non-blocking
		// descriptors are utilized.
		if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
			// Set up a standard poll matching the original C code
			// timeout window
			pfd := []unix.PollFd{{
				Fd:     int32(t.fd),
				Events: unix.POLLOUT,
			}}
			// 60,000ms timeout window matching old TIMEOUT definition
			count, pollErr := unix.Poll(pfd, 60000)
			if pollErr != nil && pollErr != unix.EINTR {
				return writtenTotal, pollErr
			}
			if count == 0 {
				return writtenTotal, errors.New("write timeout tun interface")
			}
			continue
		}

		return writtenTotal, err
	}

	return writtenTotal, nil
}
