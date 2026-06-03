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
	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/kernel"
	"github.com/markkurossi/trace"
)

func main() {
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to `file`")
	verbose := flag.Bool("v", false, "verbose output")
	ktrace := flag.Bool("ktrace", false, "kernel trace")
	cputrace := flag.Bool("cputrace", false, "CPU trace")
	color := flag.Bool("color", false, "turn on color output")
	fsroot := flag.String("fsroot", "", "filesystem root")
	objdump := flag.Bool("D", false, "disassemble")
	bios := flag.String("bios", "", "the firmwire; enables system emulation")
	kern := flag.String("kernel", "", "the kernel; enables system emulation")
	initrd := flag.String("initrd", "", "the init filesystem")
	symbols := flag.String("symbols", "", "kernel System.map")
	logger := flag.String("log", "", "logger unix domain socket")
	cooked := flag.Bool("cooked", false, "don't enable raw terminal mode")
	flag.Parse()

	log.SetFlags(0)

	if len(*logger) > 0 {
		lc, err := trace.NewClient(*logger)
		if err != nil {
			log.Fatalf("could not connect to logger %v: %v\n", *logger, err)
		}
		log.SetOutput(lc)
		fmt.Println("Remote logger enabled")
	}

	params := kernel.Params{
		Verbose:  *verbose,
		Ktrace:   *ktrace,
		CPUtrace: *cputrace,
		Profile:  len(*cpuprofile) > 0,
		Color:    *color,
		Cooked:   *cooked,
		FSRoot:   *fsroot,
	}

	if *objdump {
		disassemble(flag.Args())
		return
	}

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

	if len(*bios) > 0 || len(*kern) > 0 {
		err := systemEmulation(params, *bios, *kern, *initrd, *symbols)
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	for _, arg := range flag.Args() {
		emu, err := emulator.New(params)
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

func disassemble(files []string) {
	for _, file := range files {
		err := isa.DecodeELF(file)
		if err != nil {
			log.Fatal(err)
		}
	}
}
