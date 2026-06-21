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
	"strings"
)

type SystemConfig struct {
	BIOS    string    `json:"bios"`
	Kernel  string    `json:"kernel"`
	Symbols string    `json:"symbols,omitempty"`
	Append  string    `json:"append"`
	Initrd  string    `json:"initrd,omitempty"`
	Drives  []*Drive  `json:"drives,omitempty"`
	Netdevs []*Netdev `json:"netdevs,omitempty"`
	Devices []*Device `json:"devices,omitempty"`
}

type Drive struct {
	ID       string `json:"id"`
	File     string `json:"file"`
	Format   string `json:"format,omitempty"`
	Readonly bool   `json:"readonly,omitempty"`
}

type Netdev struct {
	ID string `json:"id"`
}

type Device struct {
	Type   string `json:"type"`
	Drive  string `json:"drive"`
	Netdev string `json:"netdev"`
	MAC    string `json:"mac,omitempty"`
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
