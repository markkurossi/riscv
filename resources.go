//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package riscv

import (
	"embed"
)

// resource holds our static emulator resources.
//
//go:embed resources/*
var Resources embed.FS
