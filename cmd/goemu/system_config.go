//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package main

import (
	"encoding/json"
	"io"
	"os"

	"github.com/markkurossi/riscv/virtio"
)

type SystemConfig struct {
	BIOS    string    `json:"bios"`
	Kernel  string    `json:"kernel"`
	Symbols string    `json:"symbols,omitempty"`
	Append  string    `json:"append"`
	Initrd  string    `json:"initrd,omitempty"`
	Drives  []*Drive  `json:"drives,omitempty"`
	Devices []*Device `json:"devices,omitempty"`
}

type Drive struct {
	ID       string `json:"id"`
	File     string `json:"file"`
	Format   string `json:"format,omitempty"`
	Readonly bool   `json:"readonly,omitempty"`
}

type Device struct {
	Type   string `json:"type"`
	Drive  string `json:"drive"`
	ID     string `json:"id,omitempty"`
	Serial string `json:"serial,omitempty"`

	drive *Drive
	blk   *virtio.Blk
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
