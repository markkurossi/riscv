//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type SystemConfig struct {
	BIOS      string    `json:"bios"`
	Kernel    string    `json:"kernel"`
	Symbols   string    `json:"symbols,omitempty"`
	Append    string    `json:"append"`
	Initrd    string    `json:"initrd,omitempty"`
	DumpDTB   string    `json:"dumpdtb,omitempty"`
	Memory    string    `json:"memory,omitempty"`
	NoGraphic bool      `json:"nographic,omitempty"`
	Drives    []*Drive  `json:"drives,omitempty"`
	Netdevs   []*Netdev `json:"netdevs,omitempty"`
	GPUs      []*GPU    `json:"gpus,omitempty"`
	Devices   []*Device `json:"devices,omitempty"`
}

type Drive struct {
	ID       string `json:"id"`
	File     string `json:"file"`
	Format   string `json:"format,omitempty"`
	Readonly bool   `json:"readonly,omitempty"`
}

type Netdev struct {
	ID         string `json:"id"`
	MAC        string `json:"mac,omitempty"`
	IP         string `json:"ip"`
	GW         string `json:"gw"`
	Hostname   string `json:"hostname,omitempty"`
	Domainname string `json:"domainname,omitempty"`
}

type GPU struct {
	ID     string `json:"id"`
	Title  string `json:"title,omitempty"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type Device struct {
	Type   string `json:"type"`
	Drive  string `json:"drive"`
	Netdev string `json:"netdev"`
	GPU    string `json:"gpu"`
	ID     string `json:"id,omitempty"`
	Serial string `json:"serial,omitempty"`
}

func parseBool(v string) (bool, error) {
	switch v {
	case "on", "true":
		return true, nil
	case "off", "false":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value '%v'", v)
	}
}

var reMem = regexp.MustCompilePOSIX(`^([[:digit:]]+)([[:alpha:]])?$`)

func ParseMem(v string) (uint64, error) {
	m := reMem.FindStringSubmatch(v)
	if m == nil {
		return 0, fmt.Errorf("invalid memory specification: %v", v)
	}
	base, err := strconv.ParseUint(m[1], 10, 64)
	if err != nil {
		return 0, err
	}
	switch m[2] {
	case "k", "K":
		base *= 1024

	case "m", "M":
		base *= 1024 * 1024

	case "g", "G":
		base *= 1024 * 1024 * 1024

	case "":

	default:
		return 0, fmt.Errorf("invalid suffix '%v'", m[2])
	}

	return base, nil
}

func ParseDrive(args string) (*Drive, error) {
	drive := new(Drive)

	for _, arg := range strings.Split(args, ",") {
		parts := strings.Split(arg, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid drive argument: %v", arg)
		}
		switch parts[0] {
		case "id":
			drive.ID = parts[1]
		case "file":
			drive.File = parts[1]
		case "format":
			drive.Format = parts[1]
		case "readonly":
			b, err := parseBool(parts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid boolean argument: %v", arg)
			}
			drive.Readonly = b
		default:
			return nil, fmt.Errorf("invalid argument: %v", arg)
		}
	}

	return drive, nil
}

func ParseGPU(args string) (*GPU, error) {
	gpu := new(GPU)
	var err error

	for _, arg := range strings.Split(args, ",") {
		parts := strings.Split(arg, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid gpu argument: %v", arg)
		}
		switch parts[0] {
		case "title":
			gpu.Title = parts[1]
		case "id":
			gpu.ID = parts[1]
		case "width":
			gpu.Width, err = strconv.Atoi(parts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid width %v: %v", parts[1], err)
			}
		case "height":
			gpu.Height, err = strconv.Atoi(parts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid height %v: %v", parts[1], err)
			}
		default:
			return nil, fmt.Errorf("invalid argument: %v", arg)
		}
	}

	return gpu, nil
}

func ParseDevice(args string) (*Device, error) {
	dev := new(Device)

	for idx, arg := range strings.Split(args, ",") {
		parts := strings.Split(arg, "=")

		if idx == 0 {
			if len(parts) != 1 {
				return nil, fmt.Errorf("invalid device type: %v", arg)
			}
			dev.Type = arg
			continue
		}
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid device argument: %v", arg)
		}
		switch parts[0] {
		case "drive":
			dev.Drive = parts[1]
		case "id":
			dev.ID = parts[1]
		case "serial":
			dev.Serial = parts[1]
		default:
			return nil, fmt.Errorf("invalid argument: %v", arg)
		}
	}

	return dev, nil
}

func ReadConfig(file string) (*SystemConfig, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	cfg := new(SystemConfig)
	err = json.Unmarshal(data, cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func (cfg *SystemConfig) Defined() bool {
	if cfg == nil {
		return false
	}
	return len(cfg.Kernel) > 0 || len(cfg.BIOS) > 0
}

func (cfg *SystemConfig) Merge(o *SystemConfig) *SystemConfig {
	if cfg == nil {
		return o
	}

	if len(o.BIOS) > 0 {
		cfg.BIOS = o.BIOS
	}
	if len(o.Kernel) > 0 {
		cfg.Kernel = o.Kernel
	}
	if len(o.Symbols) > 0 {
		cfg.Symbols = o.Symbols
	}
	if len(o.Append) > 0 {
		cfg.Append = o.Append
	}
	if len(o.Initrd) > 0 {
		cfg.Initrd = o.Initrd
	}
	if len(o.DumpDTB) > 0 {
		cfg.DumpDTB = o.DumpDTB
	}
	if len(o.Memory) > 0 {
		cfg.Memory = o.Memory
	}
	if o.NoGraphic {
		cfg.NoGraphic = true
	}
	cfg.Drives = append(cfg.Drives, o.Drives...)
	cfg.Devices = append(cfg.Devices, o.Devices...)

	return cfg
}

func (cfg *SystemConfig) Drive(id string) *Drive {
	for _, drive := range cfg.Drives {
		if drive.ID == id {
			return drive
		}
	}
	return nil
}

func (cfg *SystemConfig) Netdev(id string) *Netdev {
	for _, netdev := range cfg.Netdevs {
		if netdev.ID == id {
			return netdev
		}
	}
	return nil
}

func (cfg *SystemConfig) GPUDev(id string) *GPU {
	for _, gpu := range cfg.GPUs {
		if gpu.ID == id {
			return gpu
		}
	}
	return nil
}
