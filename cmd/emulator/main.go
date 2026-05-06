//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/pprof"

	"github.com/markkurossi/riscv/emulator"
)

func main() {
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to `file`")
	ktrace := flag.Bool("ktrace", false, "kernel trace")
	flag.Parse()

	if len(*cpuprofile) > 0 {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	for _, arg := range flag.Args() {
		emu, err := emulator.New(*ktrace, len(*cpuprofile) > 0)
		if err != nil {
			log.Fatalf("failed to create emulator: %v", err)
		}
		err = emu.LoadELF(arg)
		if err != nil {
			log.Fatalf("failed to load %v: %v", arg, err)
		}
		err = emu.Run(flag.Args(), os.Environ())
		if err != nil {
			fmt.Printf("run: %v\n", err)
			return
		}
	}
}
