//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package dev

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/markkurossi/riscv/isa"
	"golang.org/x/term"
)

type UART struct {
	Hart   isa.Hart
	Start  uint64
	End    uint64
	Plic   *PLIC
	IRQ    uint32
	Color  bool
	Cooked bool

	oldState *term.State

	m          sync.Mutex
	inputAvail atomic.Bool
	input      []byte

	// Registers
	EIR uint8 // Interrupt Enable Register.
	FCR uint8 // FIFO Control Register.
	LCR uint8 // Line Control Register.
	MCR uint8 // Modem Control Register.
	SCR uint8 // Scratchpad Register.

	// Divisor Latch tracking variables
	DLL uint8
	DLM uint8

	// Bit 0: Receiver Data Available
	// Bit 1: Transmitter Holding Register Empty
	isrPending uint8
}

func NewUART(hart isa.Hart, start, size uint64, plic *PLIC, irq uint32,
	color, cooked bool) *UART {

	uart := &UART{
		Hart:   hart,
		Start:  start,
		End:    start + size,
		Plic:   plic,
		IRQ:    irq,
		Color:  color,
		Cooked: cooked,

		// Initialize the standard default baud latches to a sane state.
		// For a 24MHz clock, a divisor of 13 matches 115200 baud.
		DLL: 13, // 0x0D
		DLM: 0,  // 0x00

		// A real 16550A also typically initializes LCR/MCR safely.
		LCR: 0x03, // 8 bits, no parity, 1 stop bit
	}
	return uart
}

func (uart *UART) Run() {
	if !uart.Cooked {
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			fmt.Printf("term.MakeRaw: %v\n", err)
			return
		}
		uart.oldState = oldState
	}

	var buf [1]byte

	for {
		n, err := os.Stdin.Read(buf[:])
		if err != nil {
			break
		}
		uart.m.Lock()
		uart.input = append(uart.input, buf[:n]...)
		if (uart.EIR & 0x01) != 0 {
			uart.isrPending |= 0x01 // Receiver data available
			if uart.Plic != nil {
				uart.Plic.Pending |= (1 << uart.IRQ)
				uart.Plic.ReevaluateInterrupts()
			}
		}
		uart.m.Unlock()

		uart.inputAvail.Store(true)

		if uart.Plic != nil && (uart.EIR&0x01) != 0 {
			uart.Plic.Pending |= (1 << uart.IRQ)
			uart.Plic.ReevaluateInterrupts()
		}
	}
}

func (uart *UART) Halt() error {
	if uart.oldState != nil {
		term.Restore(int(os.Stdin.Fd()), uart.oldState)
	}
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
		// Check if DLAB bit (Bit 7) is set in LCR
		if (uart.LCR & 0x80) != 0 {
			return uart.DLL, nil
		}

		var v byte = 0xff

		uart.m.Lock()
		if len(uart.input) > 0 {
			v = uart.input[0]
			uart.input = uart.input[1:]
			if len(uart.input) == 0 {
				uart.inputAvail.Store(false)
				uart.isrPending &^= 0x01 // Clear receiver interrupt flag

				if uart.isrPending == 0 && uart.Plic != nil {
					uart.Plic.Pending &^= (1 << uart.IRQ)
					uart.Plic.ReevaluateInterrupts()
				}
			}
		}
		uart.m.Unlock()

		return v, nil

	case 1:
		// Check if DLAB bit (Bit 7) is set in LCR
		if (uart.LCR & 0x80) != 0 {
			return uart.DLM, nil
		}

		// Return Interrupt Enable Register
		return uart.EIR, nil

	case 2:
		// Interrupt Identification Register (IIR)
		uart.m.Lock()
		defer uart.m.Unlock()

		var iir byte
		if uart.FCR&0x01 != 0 {
			// 16550A standard values: FIFO enabled (0xC0)
			iir = 0xC0
		}

		if (uart.isrPending & 0x02) != 0 {
			// Transmitter Holding Register Empty has priority or is active
			iir |= 0x02
			// Reading the IIR register automatically clears the THRE interrupt status!
			uart.isrPending &^= 0x02

			// If no other conditions remain, lower the PLIC line
			if uart.isrPending == 0 && uart.Plic != nil {
				uart.Plic.Pending &^= (1 << uart.IRQ)
				uart.Plic.ReevaluateInterrupts()
			}
			return iir, nil
		}

		if uart.inputAvail.Load() {
			// Received Data Available interrupt (0x04)
			iir |= 0x04
			return iir, nil
		}

		// Bit 0 is 1 if NO interrupt is pending
		iir |= 0x01
		return iir, nil

	case 3: // LCR Read
		return uart.LCR, nil

	case 4: // MCR Read
		return uart.MCR, nil

	case 5:
		// Transmitter Empty (0x20)  + Transmitter Idle (0x40).
		var status byte = 0x60

		if uart.inputAvail.Load() {
			// Data ready.
			status |= 0x01
		}
		return status, nil

	case 6: // MSR (Modem Status Register)
		// Standard terminal connections:
		// Bit 4: CTS (Clear to Send) = 1
		// Bit 5: DSR (Data Set Ready) = 1
		// Bit 7: DCD (Data Carrier Detect) = 1 -> CRUCIAL FOR FreeBSD PPS
		return 0xB0, nil

	case 7:
		return uart.SCR, nil
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

	uart.m.Lock()
	defer uart.m.Unlock()

	switch paddr - uart.Start {
	case 0:
		// Check if DLAB bit (Bit 7) is set in LCR
		if (uart.LCR & 0x80) != 0 {
			uart.DLL = byte(v)
			return nil
		}

		if uart.Color {
			uart.Hart.ColorOn()
			os.Stdout.Write([]byte{byte(v)})
			uart.Hart.ColorOff()
		} else {
			os.Stdout.Write([]byte{byte(v)})
		}

		// Check if Transmitter Holding Register Ready Interrupt is enabled
		if (uart.EIR & 0x02) != 0 {
			uart.isrPending |= 0x02 // Mark THRE interrupt active internally
			if uart.Plic != nil {
				uart.Plic.Pending |= (1 << uart.IRQ)
				uart.Plic.ReevaluateInterrupts()
			}
		}

	case 1:
		// Check if DLAB bit (Bit 7) is set in LCR
		if (uart.LCR & 0x80) != 0 {
			uart.DLM = byte(v)
			return nil
		}
		uart.EIR = byte(v)

	case 2:
		uart.FCR = byte(v)

	case 3:
		uart.LCR = byte(v)

	case 4:
		uart.MCR = byte(v)

	case 5: // LSR Write (ignored or clears errors on standard chips)
		return nil

	case 6: // MSR Write (read-only register physically, writes are ignored)
		return nil

	case 7:
		uart.SCR = byte(v)
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
