//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package main

import (
	"flag"
	"log"
	"os"

	"github.com/markkurossi/riscv/emulator"
)

func main() {
	flag.Parse()

	for _, arg := range flag.Args() {
		emu := emulator.New()
		err := emu.LoadELF(arg)
		if err != nil {
			log.Fatalf("failed to load %v: %v", arg, err)
		}
		err = emu.Run(flag.Args(), os.Environ())
		if err != nil {
			log.Fatal(err)
		}
	}
}
