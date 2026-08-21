//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package dev

import (
	"time"

	"github.com/markkurossi/riscv/isa"
)

// GoldfishRTC implements real-time clock device.
type GoldfishRTC struct {
	Hart  isa.Hart
	Start uint64
	End   uint64
}

// Halt implements MMIO.Halt.
func (rtc *GoldfishRTC) Halt() error {
	return nil
}

// Contains implements MMIO.Contains.
func (rtc *GoldfishRTC) Contains(paddr, size uint64) bool {
	return paddr >= rtc.Start && paddr+size <= rtc.End
}

// Load8 implements MMIO.Load8.
func (rtc *GoldfishRTC) Load8(paddr uint64) (uint8, error) {
	return 0, nil
}

// Load16 implements MMIO.Load16.
func (rtc *GoldfishRTC) Load16(paddr uint64) (uint16, error) {
	return 0, nil
}

// Load32 implements MMIO.Load32.
func (rtc *GoldfishRTC) Load32(paddr uint64) (uint32, error) {
	if paddr < rtc.Start {
		return 0, rtc.Hart.Trap(isa.CauseLoadPageFault, paddr, nil)
	}

	offset := paddr - rtc.Start

	// Fetch the absolute wall-clock time from the real host machine.
	nowNano := uint64(time.Now().UnixNano())

	switch offset {
	case 0x00: // TIME_LOW
		return uint32(nowNano & 0xffffffff), nil
	case 0x04: // TIME_HIGH
		return uint32(nowNano >> 32), nil
	}

	return 0, nil
}

// Load64 implements MMIO.Load64.
func (rtc *GoldfishRTC) Load64(paddr uint64) (uint64, error) {
	return 0, nil
}

// The Goldfish RTC hardware registers are read-only for standard
// timekeeping.

// Store8 implements MMIO.Store8.
func (rtc *GoldfishRTC) Store8(paddr uint64, v uint8) error {
	return nil
}

// Store16 implements MMIO.Store16.
func (rtc *GoldfishRTC) Store16(paddr uint64, v uint16) error {
	return nil
}

// Store32 implements MMIO.Store32.
func (rtc *GoldfishRTC) Store32(paddr uint64, v uint32) error {
	return nil
}

// Store64 implements MMIO.Store64.
func (rtc *GoldfishRTC) Store64(paddr uint64, v uint64) error {
	return nil
}
