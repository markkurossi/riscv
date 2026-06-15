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
	"github.com/markkurossi/riscv/logger"
	"golang.org/x/term"
)

type UARTRReg uint8

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
	return fmt.Sprintf("UART-%02x", r)
}

type UARTWReg uint8

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
	return fmt.Sprintf("UART-%02x", r)
}

// IER bit descriptions.
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

// FCR: FIFO Control Register bit descriptions.
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

// MCR: Modem control register bit descriptions.
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

type UART struct {
	logger.Logger
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

func NewUART(hart isa.Hart, start, size uint64, plic *PLIC, irq uint32,
	color, cooked bool) *UART {

	uart := &UART{
		Logger: logger.Logger{
			Name:  "UART",
			Level: logger.Error,
		},
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
		if (uart.IER & 0x01) != 0 {
			uart.isrPending |= 0x01 // Receiver data available
			if uart.Plic != nil {
				uart.Plic.Pending |= (1 << uart.IRQ)
				uart.Plic.ReevaluateInterrupts()
			}
		}
		uart.inputAvail.Store(true)
		uart.m.Unlock()
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
	reg := UARTRReg(paddr - uart.Start)
	uart.Debugf("Load8(%v)", reg)

	switch reg {
	case RRegRBR: // Receive Buffer
		if uart.LCR&LCRDLABBit != 0 {
			// DLL: Divisor Latch LSB
			return uart.DLL, nil
		}

		var v byte = 0xff

		uart.m.Lock()
		if len(uart.input) > 0 {
			v = uart.input[0]
			uart.input = uart.input[1:]
		}

		if len(uart.input) == 0 {
			uart.inputAvail.Store(false)
			uart.isrPending &^= 0x01 // Clear receiver interrupt flag

			if uart.isrPending == 0 && uart.Plic != nil {
				uart.Plic.Pending &^= (1 << uart.IRQ)
				uart.Plic.ReevaluateInterrupts()
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
			if uart.isrPending == 0 && uart.Plic != nil {
				uart.Plic.Pending &^= (1 << uart.IRQ)
				uart.Plic.ReevaluateInterrupts()
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
		return 0, nil

	case RRegSCR: // Scratch
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

	reg := UARTWReg(paddr - uart.Start)
	uart.Debugf("Store8(%v, 0x%02x)", reg, v)

	switch reg {
	case WRegTHR:
		if uart.LCR&LCRDLABBit != 0 {
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

		// Check if CPU wants to know that the THR is empty via an
		// interrupt.
		uart.checkInterrupts()

	case WRegIER:
		if uart.LCR&LCRDLABBit != 0 {
			uart.DLM = byte(v)

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
		uart.IER = byte(v)

		// Check if CPU expects state changes via interrupts.
		uart.checkInterrupts()

	case WRegFCR:
		uart.FCR = byte(v)
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
		uart.LCR = byte(v)
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
		uart.MCR = byte(v)
		if v != 0 {
			uart.Infof("MCR: Modem Control Register store:")
			for i, desc := range MCRBits {
				if v&(1<<i) != 0 {
					uart.Infof("  %v: %v", i, desc)
				}
			}
		}

	case WRegSCR:
		uart.SCR = byte(v)
		uart.Infof("SCR: %08b", v)
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

func (uart *UART) checkInterrupts() {

	// Check if Transmitter Holding Register Ready Interrupt is
	// enabled.
	if (uart.IER & 0x02) != 0 {
		// Mark THRE interrupt active internally
		uart.isrPending |= 0x02

		if uart.Plic != nil {
			uart.Plic.Pending |= (1 << uart.IRQ)
			uart.Plic.ReevaluateInterrupts()
		}
	}

}
