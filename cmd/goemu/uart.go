//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package main

import (
	"fmt"

	"github.com/markkurossi/riscv/isa"
)

type UART struct {
	Start uint64
	End   uint64
	Color bool

	// Interrupt Enable Register
	EIR uint8

	// FIFO Control Register
	FCR uint8

	// Line Control Register
	LCR uint8

	// Modem Control Register
	MCR uint8
}

func (uart *UART) Contains(paddr uint64) bool {
	return paddr >= uart.Start && paddr < uart.End
}

func (uart *UART) Load8(paddr uint64) (uint8, error) {
	if paddr < uart.Start {
		return 0, isa.NewTrap(0, 0, isa.CauseStorePageFault, paddr, nil)
	}
	switch paddr - uart.Start {
	case 5:
		// Return 0x20 (Transmitter Empty) + 0x40 (Transmitter Idle).
		return 0x60, nil
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
		return isa.NewTrap(0, 0, isa.CauseStorePageFault, paddr, nil)
	}
	switch paddr - uart.Start {
	case 0:
		if uart.Color {
			fmt.Printf("\x1b[106;30m%c\x1b[0m", byte(v))
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
