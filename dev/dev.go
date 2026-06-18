//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

// Package dev implements memory-mapped devices.
package dev

import (
	"github.com/markkurossi/riscv/mmu"
)

var (
	_ mmu.MMIO = &CLINT{}
	_ mmu.MMIO = &PLIC{}
	_ mmu.MMIO = &GoldfishRTC{}
	_ mmu.MMIO = &Syscon{}
	_ mmu.MMIO = &UART{}
)
