//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package dev

import (
	"fmt"
	"sync"
	"time"

	"github.com/markkurossi/riscv/isa"
)

var (
	_ UARTPeer = &UARTNMEA{}
)

// UARTNMEA implements GPS NMEA date sensor.
type UARTNMEA struct {
	Hart isa.Hart

	m       sync.Mutex
	halted  bool
	input   []byte
	pending bool
}

// Run implements UARTPeer.Run.
func (nmea *UARTNMEA) Run(uart *UART) {
	ticker := time.NewTicker(1 * time.Second)

	for !nmea.halted {
		<-ticker.C
		nmea.pending = true
		uart.InputAvailable()
	}
}

// Halt implements UARTPeer.Halt.
func (nmea *UARTNMEA) Halt() error {
	nmea.halted = true
	return nil
}

// Receive implements UARTPeer.Receive.
func (nmea *UARTNMEA) Receive() (byte, bool) {
	nmea.m.Lock()
	defer nmea.m.Unlock()

	var v byte = 0xff

	if len(nmea.input) == 0 {
		if !nmea.pending {
			return 0xff, false
		}
		nmea.input = []byte(nmea.formatDate())
	}

	v = nmea.input[0]
	nmea.input = nmea.input[1:]

	return v, len(nmea.input) > 0
}

func (nmea *UARTNMEA) formatDate() string {
	utc := time.Now().UTC()
	timeStr := utc.Format("150405.00") // HHMMSS.ss
	dateStr := utc.Format("020106")    // DDMMYY

	// Dummy fixed coordinates (e.g., 0000.000,N,00000.000,E).
	raw := fmt.Sprintf("GPRMC,%s,A,0000.000,N,00000.000,E,0.0,0.0,%s,,,A",
		timeStr, dateStr)

	// Calculate NMEA XOR checksum.
	var checksum byte
	for i := 0; i < len(raw); i++ {
		checksum ^= raw[i]
	}

	return fmt.Sprintf("$%s*%02X\r\n", raw, checksum)
}

// Send implements UARTPeer.Send.
func (nmea *UARTNMEA) Send(b byte) {
}
