//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package dev

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/logger"
)

const (
	// UARTSize defines the UART MMIO size.
	UARTSize = 256
)

// UARTRReg implements UART read registers.
type UARTRReg uint8

// Readable registers.
const (
	RRegRBR UARTRReg = iota // receive buffer
	RRegIER                 // interrupt enable
	RRegIIR                 // interrupt identification
	RRegLCR                 // line control
	RRegMCR                 // modem control
	RRegLSR                 // line status
	RRegMSR                 // modem status
	RRegSCR                 // scratch
)

var uartRRegs = map[UARTRReg]string{
	RRegRBR: "RBR",
	RRegIER: "IER",
	RRegIIR: "IIR",
	RRegLCR: "LCR",
	RRegMCR: "MCR",
	RRegLSR: "LSR",
	RRegMSR: "MSR",
	RRegSCR: "SCR",
}

func (r UARTRReg) String() string {
	name, ok := uartRRegs[r]
	if ok {
		return name
	}
	return fmt.Sprintf("%02x", uint8(r))
}

// UARTWReg implements UART write registers.
type UARTWReg uint8

// Writable registers.
const (
	WRegTHR         UARTWReg = iota // transmitter holding
	WRegIER                         // interrupt enable
	WRegFCR                         // FIFO control
	WRegLCR                         // line control
	WRegMCR                         // modem control
	WRegFactoryTest                 // factory test
	WRegNotUsed                     // not used
	WRegSCR                         // scratch
)

var uartWRegs = map[UARTWReg]string{
	WRegTHR:         "THR",          // Transmitter Holding
	WRegIER:         "IER",          // Interrupt Enable
	WRegFCR:         "FCR",          // FIFO Control
	WRegLCR:         "LCR",          // Line Control
	WRegMCR:         "MCR",          // Modem Control
	WRegFactoryTest: "factory test", // Factory test
	WRegNotUsed:     "not used",     // Not used
	WRegSCR:         "SCR",          // Scratch
}

func (r UARTWReg) String() string {
	name, ok := uartWRegs[r]
	if ok {
		return name
	}
	return fmt.Sprintf("%02x", uint8(r))
}

// IERBits provides mapping from Interrupt Enable Register bits to
// their descriptions.
var IERBits = []string{
	"Received data available",
	"Transmitter holding register empty",
	"Receiver line status register change",
	"Modem status register change",
	"Sleep mode (16750 only)",
	"Low power mode (16750 only)",
	"reserved",
	"reserved",
}

// FCRBits provide mapping from FIFO Control Register bit to their
// descriptions.
var FCRBits = []string{
	"Enable FIFO’s",
	"Clear receive FIFO",
	"Clear transmit FIFO",
	"Select DMA mode 1",
	"Reserved",
	"Enable 64 byte FIFO (16750)",
}

// LCR: line control register (R/W)
const (
	LCRDataBits       = 0b00000011
	LCRStopBit        = 0b00000100
	LCRParityBits     = 0b00111000
	LCRBreakSignalBit = 0b01000000
	LCRDLABBit        = 0b10000000
)

// MCRBits provide mapping from Modem Control Register bits to their
// descriptions.
var MCRBits = []string{
	"Data terminal ready",
	"Request to send",
	"Auxiliary output 1",
	"Auxiliary output 2",
	"Loopback mode",
	"Autoflow control (16750 only)",
	"Reserved",
	"Reserved",
}

// MSR Bits.
const (
	MsrCTS = 1 << 4 // Clear To Send
	MsrDSR = 1 << 5 // Data Set Ready
	MsrRI  = 1 << 6 // Ring Indicator
	MsrDCD = 1 << 7 // Data Carrier Detect
)

// UART implements the UART 16550A universal asynchronous
// receiver-transmitter.
type UART struct {
	logger.Log
	Hart  isa.Hart
	Start uint64
	End   uint64
	Plic  *PLIC
	IRQ   uint32

	Peer       UARTPeer
	m          sync.Mutex
	inputAvail atomic.Bool

	// Registers
	IER uint8 // Interrupt Enable
	IIR uint8 // Interrupt Identification
	FCR uint8 // FIFO Control
	LCR uint8 // Line Control
	MCR uint8 // Modem Control
	SCR uint8 // Scratch
	DLL uint8 // Divisor Latch LSB
	DLM uint8 // Divisor Latch MSB

	// Bit 0: Receiver Data Available
	// Bit 1: Transmitter Holding Register Empty
	isrPending uint8
}

// UARTPeer implements the peer device.
type UARTPeer interface {
	Run(uart *UART)
	Halt() error
	Receive() (b byte, more bool)
	Send(b byte)
}

// NewUART creates a new UART.
func NewUART(hart isa.Hart, name string, start uint64, plic *PLIC, irq uint32,
	peer UARTPeer) *UART {

	uart := &UART{
		Log: logger.Log{
			Name:  name,
			Level: logger.Error,
		},
		Hart:  hart,
		Start: start,
		End:   start + UARTSize,
		Plic:  plic,
		IRQ:   irq,
		Peer:  peer,

		// Initialize the standard default baud latches to a sane state.
		// For a 24MHz clock, a divisor of 13 matches 115200 baud.
		DLL: 13, // 0x0D
		DLM: 0,  // 0x00

		// A real 16550A also typically initializes LCR/MCR safely.
		LCR: 0x03, // 8 bits, no parity, 1 stop bit
	}

	plic.IRQs[irq] = "UART"

	return uart
}

// Halt implements MMIO.Halt.
func (uart *UART) Halt() error {
	return uart.Peer.Halt()
}

// Contains implements MMIO.Contains.
func (uart *UART) Contains(paddr, size uint64) bool {
	return paddr >= uart.Start && paddr+size <= uart.End
}

// Load8 implements MMIO.Load8.
func (uart *UART) Load8(paddr uint64) (uint8, error) {
	if paddr < uart.Start {
		return 0, uart.Hart.Trap(isa.CauseStorePageFault, paddr, nil)
	}
	reg := UARTRReg(paddr - uart.Start)
	uart.Debugf("Load8(%v)", reg)

	switch reg {
	case RRegRBR: // Receive Buffer
		if uart.LCR&LCRDLABBit != 0 {
			// DLL: Divisor Latch LSB
			return uart.DLL, nil
		}

		v, avail := uart.Peer.Receive()

		uart.m.Lock()
		if !avail {
			uart.inputAvail.Store(false)
			uart.isrPending &^= 0x01 // Clear receiver interrupt flag
			if uart.isrPending == 0 {
				uart.Plic.SetInterruptRequest(uart.IRQ, false)
			}
		}
		uart.m.Unlock()

		return v, nil

	case RRegIER: // Interrupt Enable
		if uart.LCR&LCRDLABBit != 0 {
			// DLL: Divisor Latch MSB
			return uart.DLM, nil
		}
		return uart.IER, nil

	case RRegIIR: // Interrupt Identification
		uart.m.Lock()
		defer uart.m.Unlock()

		// 16550A standard values: FIFO enabled (0xC0)
		var iir byte = 0xC0

		if (uart.isrPending & 0x02) != 0 {
			// Transmitter Holding Register Empty has priority or is active
			iir |= 0x02
			// Reading the IIR register automatically clears the THRE
			// interrupt status!
			uart.isrPending &^= 0x02

			// If no other conditions remain, lower the PLIC line
			if uart.isrPending == 0 {
				uart.Plic.SetInterruptRequest(uart.IRQ, false)
			}
			uart.Infof("IIR read: %08b: THR empty", iir)
			return iir, nil
		}

		if uart.inputAvail.Load() {
			// Received Data Available interrupt (0x04)
			iir |= 0x04
			uart.Infof("IIR read: %08b: Received data available", iir)
			return iir, nil
		}

		// Bit 0 is 1 if NO interrupt is pending
		iir |= 0x01
		uart.Infof("IIR read: %08b: no interrupt pending", iir)
		return iir, nil

	case RRegLCR: // Line Control
		return uart.LCR, nil

	case RRegMCR: // Modem Control
		return uart.MCR, nil

	case RRegLSR: // Line Status
		// THR is empty (0x20) + THR is empty, and line is idle (0x40).
		var status byte = 0x60

		if uart.inputAvail.Load() {
			// Data ready.
			status |= 0x01
		}
		return status, nil

	case RRegMSR: // Modem Status
		uart.Infof("MSR read")
		var msr uint8 = MsrCTS | MsrDSR | MsrDCD
		return msr, nil

	case RRegSCR: // Scratch
		return uart.SCR, nil

	}
	return 0, nil
}

// Load16 implements MMIO.Load16.
func (uart *UART) Load16(paddr uint64) (uint16, error) {
	return 0, nil
}

// Load32 implements MMIO.Load32.
func (uart *UART) Load32(paddr uint64) (uint32, error) {
	return 0, nil
}

// Load64 implements MMIO.Load64.
func (uart *UART) Load64(paddr uint64) (uint64, error) {
	return 0, nil
}

// Store8 implements MMIO.Store8.
func (uart *UART) Store8(paddr uint64, v uint8) error {
	if paddr < uart.Start {
		return uart.Hart.Trap(isa.CauseStorePageFault, paddr, nil)
	}

	uart.m.Lock()
	defer uart.m.Unlock()

	reg := UARTWReg(paddr - uart.Start)
	uart.Debugf("Store8(%v, 0x%02x)", reg, v)

	switch reg {
	case WRegTHR:
		if uart.LCR&LCRDLABBit != 0 {
			uart.DLL = v
			return nil
		}
		uart.Peer.Send(v)

		// Check if CPU wants to know that the THR is empty via an
		// interrupt.
		uart.checkInterrupts()

	case WRegIER:
		if uart.LCR&LCRDLABBit != 0 {
			uart.DLM = v

			// Baud rate: ClockFrequency/16*Divisor

			divisor := uint32(uart.DLM)<<8 + uint32(uart.DLL)
			baudRate := 24000000 / (16 * divisor)
			uart.Infof("Baud Rate %v", baudRate)

			return nil
		}
		if v != 0 {
			uart.Infof("IER: Interrupt Enable Register store:")
			for i, desc := range IERBits {
				if v&(1<<i) != 0 {
					uart.Infof("  %v: %v", i, desc)
				}
			}
		}
		uart.IER = v

		// Check if CPU expects state changes via interrupts.
		uart.checkInterrupts()

	case WRegFCR:
		uart.FCR = v
		if v != 0 {
			uart.Infof("FCR: FIFO Control Register store:")
			for i, desc := range FCRBits {
				if v&(1<<i) != 0 {
					uart.Infof("  %v: %v", i, desc)
				}
			}
			var trigger string
			switch v >> 6 {
			case 0:
				trigger = "1 byte"
			case 1:
				trigger = "4 bytes"
			case 2:
				trigger = "8 bytes"
			case 3:
				trigger = "14 bytes"
			}
			uart.Infof("   : FIFO interrupt trigger: %v", trigger)
		}

	case WRegLCR:
		uart.LCR = v
		uart.Infof("LCR: Line Control Register store:")
		uart.Infof("   : %d data bits", v&0b11+5)
		uart.Infof("   : %d stop bits", v>>2&0b1+1)
		if v&0b00111000 != 0 {
			uart.Infof("   : parity: %08b", v&0b00111000)
		}
		if v&0b01000000 != 0 {
			uart.Infof("  6: Break signal enabled")
		}
		if v&0b10000000 != 0 {
			uart.Infof("  7: DLAB : DLL and DLM accessible")
		} else {
			uart.Infof("  7: DLAB : RBR, THR and IER accessible")
		}

	case WRegMCR:
		uart.MCR = v
		if v != 0 {
			uart.Infof("MCR: Modem Control Register store:")
			for i, desc := range MCRBits {
				if v&(1<<i) != 0 {
					uart.Infof("  %v: %v", i, desc)
				}
			}
		}

	case WRegSCR:
		uart.SCR = v
		uart.Infof("SCR: %08b", v)
	}
	return nil
}

// Store16 implements MMIO.Store16.
func (uart *UART) Store16(paddr uint64, v uint16) error {
	fmt.Printf("ROM.store16: %x = %v\n", paddr, v)
	return nil
}

// Store32 implements MMIO.Store32.
func (uart *UART) Store32(paddr uint64, v uint32) error {
	fmt.Printf("ROM.store32: %x = %v\n", paddr, v)
	return nil
}

// Store64 implements MMIO.Store64.
func (uart *UART) Store64(paddr uint64, v uint64) error {
	fmt.Printf("ROM.store64: %x = %v\n", paddr, v)
	return nil
}

// InputAvailable notifies that UART peer has input.
func (uart *UART) InputAvailable() {
	uart.m.Lock()
	defer uart.m.Unlock()

	if (uart.IER & 0x01) != 0 {
		uart.isrPending |= 0x01 // Receiver data available
		uart.Plic.SetInterruptRequest(uart.IRQ, true)
	}
	uart.inputAvail.Store(true)
}

func (uart *UART) checkInterrupts() {

	// Check if Transmitter Holding Register Ready Interrupt is
	// enabled.
	if (uart.IER & 0x02) != 0 {
		// Mark THRE interrupt active internally
		uart.isrPending |= 0x02
		uart.Plic.SetInterruptRequest(uart.IRQ, true)
	}
}
