//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package dev

import (
	"log"
	"os"
	"sync"

	"github.com/markkurossi/riscv/isa"
	"golang.org/x/term"
)

var (
	_ UARTPeer = &UARTConsole{}
)

// UARTConsole implements UART console device.
type UARTConsole struct {
	Hart   isa.Hart
	Color  bool
	Cooked bool

	m        sync.Mutex
	oldState *term.State
	input    []byte
}

// Run implements UARTPeer.Run.
func (cons *UARTConsole) Run(uart *UART) {
	if !cons.Cooked {
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			log.Printf("term.MakeRaw: %v", err)
			return
		}
		cons.oldState = oldState
	}

	var buf [1]byte

	for {
		n, err := os.Stdin.Read(buf[:])
		if err != nil {
			break
		}
		cons.m.Lock()
		cons.input = append(cons.input, buf[:n]...)
		cons.m.Unlock()

		uart.InputAvailable()
	}
}

// Halt implements UARTPeer.Halt.
func (cons *UARTConsole) Halt() error {
	if cons.oldState != nil {
		term.Restore(int(os.Stdin.Fd()), cons.oldState)
	}
	return nil
}

// Receive implements UARTPeer.Receive.
func (cons *UARTConsole) Receive() (byte, bool) {
	cons.m.Lock()
	defer cons.m.Unlock()

	if len(cons.input) == 0 {
		return 0xff, false
	}

	v := cons.input[0]
	cons.input = cons.input[1:]

	return v, len(cons.input) > 0
}

// Send implements UARTPeer.Send.
func (cons *UARTConsole) Send(b byte) {
	if cons.Color {
		cons.Hart.ColorOn()
		os.Stdout.Write([]byte{b})
		cons.Hart.ColorOff()
	} else {
		os.Stdout.Write([]byte{b})
	}
}
