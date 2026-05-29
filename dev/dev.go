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
	_ mmu.ROM = &CLINT{}
	_ mmu.ROM = &PLIC{}
	_ mmu.ROM = &GoldfishRTC{}
	_ mmu.ROM = &UART{}
)
