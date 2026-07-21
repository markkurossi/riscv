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
	"strings"

	"github.com/markkurossi/riscv/emulator"
	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/kernel"
	"github.com/markkurossi/trace"
)

type drives []*Drive

func (arg *drives) String() string {
	return fmt.Sprintf("%v", *arg)
}

func (arg *drives) Set(value string) error {
	drive, err := ParseDrive(value)
	if err != nil {
		return err
	}
	*arg = append(*arg, drive)
	return nil
}

type devices []*Device

func (arg *devices) String() string {
	return fmt.Sprintf("%v", *arg)
}

func (arg *devices) Set(value string) error {
	dev, err := ParseDevice(value)
	if err != nil {
		return err
	}
	*arg = append(*arg, dev)
	return nil
}

var (
	argCfg     *SystemConfig = new(SystemConfig)
	argDrives  drives
	argDevices devices
)

func init() {
}

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
	bootargs := flag.String("append", "", "kernel boot args")
	symbols := flag.String("symbols", "", "kernel System.map")
	logger := flag.String("log", "", "logger unix domain socket")
	cooked := flag.Bool("cooked", false, "don't enable raw terminal mode")
	csr802 := flag.String("csr802", ",csr802", "CSR802 CPU profiling filename")
	gpu := flag.String("gpu", "", "graphics device")
	htif := flag.Bool("htif", false, "enable host target interface")

	flag.Var(&argDrives, "drive", "configure drive")
	flag.Var(&argDevices, "device", "configure device")

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
		CSR802:   *csr802,
		Profile:  len(*cpuprofile) > 0,
		Color:    *color,
		Cooked:   *cooked,
		FSRoot:   *fsroot,
	}

	argCfg.BIOS = *bios
	argCfg.Kernel = *kern
	argCfg.Symbols = *symbols
	argCfg.Initrd = *initrd
	argCfg.Append = *bootargs

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

	// Process args for .goemu configs.

	var systemConfig *SystemConfig
	var rest []string

	for _, arg := range flag.Args() {
		if !strings.HasSuffix(arg, ".goemu") {
			rest = append(rest, arg)
			continue
		}
		cfg, err := ReadConfig(arg)
		if err != nil {
			log.Fatalf("could not read config %v: %v", arg, err)
		}
		systemConfig = systemConfig.Merge(cfg)
	}

	// Merge argument configs.
	systemConfig = systemConfig.Merge(argCfg)
	systemConfig.Drives = append(systemConfig.Drives, argDrives...)
	systemConfig.Devices = append(systemConfig.Devices, argDevices...)

	if len(*gpu) > 0 {
		gpu, err := ParseGPU(*gpu)
		if err != nil {
			log.Fatal(err)
		}
		systemConfig.GPUs = append(systemConfig.GPUs, gpu)
		systemConfig.Devices = append(systemConfig.Devices, &Device{
			Type: "virtio-gpu-device",
			GPU:  gpu.ID,
		})
	}

	// If any critical system emulation parameters are set, start
	// system emulation.
	if systemConfig.Defined() {
		err := systemEmulation(*htif, params, systemConfig, rest)
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
