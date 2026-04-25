//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package main

import (
	"flag"
	"log"

	"github.com/markkurossi/riscv/isa"
)

func main() {
	flag.Parse()

	for _, arg := range flag.Args() {
		err := disassembleFile(arg)
		if err != nil {
			log.Fatalf("failed to disassemble %v: %v", arg, err)
		}
	}
}

func disassembleFile(name string) error {
	err := isa.DecodeELF(name)
	if err != nil {
		return err
	}
	return nil
}
