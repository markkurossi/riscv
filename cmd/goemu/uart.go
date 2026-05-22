//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package main

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/markkurossi/riscv/isa"
	"golang.org/x/term"
)

type UART struct {
	Hart  isa.Hart
	Start uint64
	End   uint64
	Color bool

	oldState *term.State

	m          sync.Mutex
	inputAvail atomic.Bool
	input      []byte

	// Interrupt Enable Register
	EIR uint8

	// FIFO Control Register
	FCR uint8

	// Line Control Register
	LCR uint8

	// Modem Control Register
	MCR uint8
}

func (uart *UART) Run() {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Printf("term.MakeRaw: %v\n", err)
		return
	}
	uart.oldState = oldState

	var buf [1]byte

	for {
		n, err := os.Stdin.Read(buf[:])
		if err != nil {
			break
		}
		uart.m.Lock()
		uart.input = append(uart.input, buf[:n]...)
		uart.m.Unlock()

		uart.inputAvail.Store(true)
	}
}

func (uart *UART) Halt() error {
	term.Restore(int(os.Stdin.Fd()), uart.oldState)
	return nil
}

func (uart *UART) Contains(paddr uint64) bool {
	return paddr >= uart.Start && paddr < uart.End
}

func (uart *UART) Load8(paddr uint64) (uint8, error) {
	if paddr < uart.Start {
		return 0, uart.Hart.Trap(isa.CauseStorePageFault, paddr, nil)
	}
	switch paddr - uart.Start {
	case 0:
		var v byte

		uart.m.Lock()
		if len(uart.input) > 0 {
			v = uart.input[0]
			uart.input = uart.input[1:]
			if len(uart.input) == 0 {
				uart.inputAvail.Store(false)
			}
		}
		uart.m.Unlock()

		return v, nil

	case 5:
		// Transmitter Empty (0x20)  + Transmitter Idle (0x40).
		var status byte = 0x60

		if uart.inputAvail.Load() {
			// Data ready.
			status |= 0x01
		}
		return status, nil
	}
	return 0, nil
}

func (uart *UART) Load16(paddr uint64) (uint16, error) {
	return 0, nil
}

func (uart *UART) Load32(paddr uint64) (uint32, error) {
	return 0, nil
}

func (uart *UART) Load64(paddr uint64) (uint64, error) {
	return 0, nil
}

func (uart *UART) Store8(paddr, v uint64) error {
	if paddr < uart.Start {
		return uart.Hart.Trap(isa.CauseStorePageFault, paddr, nil)
	}
	switch paddr - uart.Start {
	case 0:
		if uart.Color {
			uart.Hart.ColorOn()
			fmt.Printf("%c", byte(v))
			uart.Hart.ColorOff()
		} else {
			fmt.Printf("%c", byte(v))
		}

	case 1:
		uart.EIR = byte(v)

	case 2:
		uart.FCR = byte(v)

	case 3:
		uart.LCR = byte(v)

	case 4:
		uart.MCR = byte(v)
	}
	return nil
}

func (uart *UART) Store16(paddr, v uint64) error {
	fmt.Printf("ROM.store16: %x = %v\n", paddr, v)
	return nil
}

func (uart *UART) Store32(paddr, v uint64) error {
	fmt.Printf("ROM.store32: %x = %v\n", paddr, v)
	return nil
}

func (uart *UART) Store64(paddr, v uint64) error {
	fmt.Printf("ROM.store64: %x = %v\n", paddr, v)
	return nil
}
