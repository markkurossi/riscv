//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package main

import (
	"time"

	"github.com/markkurossi/riscv/isa"
)

type GoldfishRTC struct {
	Hart  isa.Hart
	Start uint64
	End   uint64
}

func (rtc *GoldfishRTC) Contains(paddr uint64) bool {
	return paddr >= rtc.Start && paddr < rtc.End
}

func (rtc *GoldfishRTC) Load8(paddr uint64) (uint8, error) {
	return 0, nil
}

func (rtc *GoldfishRTC) Load16(paddr uint64) (uint16, error) {
	return 0, nil
}

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

func (rtc *GoldfishRTC) Load64(paddr uint64) (uint64, error) {
	return 0, nil
}

// The Goldfish RTC hardware registers are read-only for standard timekeeping

func (rtc *GoldfishRTC) Store8(paddr, v uint64) error {
	return nil
}

func (rtc *GoldfishRTC) Store16(paddr, v uint64) error {
	return nil
}

func (rtc *GoldfishRTC) Store32(paddr, v uint64) error {
	return nil
}

func (rtc *GoldfishRTC) Store64(paddr, v uint64) error {
	return nil
}
