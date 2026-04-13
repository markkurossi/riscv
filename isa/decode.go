//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package isa

import (
	"encoding/binary"
	"fmt"
)

var (
	bo = binary.LittleEndian
)

// Decode decodes RISC-V instructions from data and returns the
// decoded program.
func Decode(data []byte) ([]Instr, error) {
	var result []Instr

	for len(data) > 0 {
		if len(data) < 2 {
			return nil, fmt.Errorf("truncated data")
		}
		opcode := data[0]
		if opcode&0b11 != 0b11 {
			return nil, fmt.Errorf("compressed instructions not supported yet")
		}

		// 32-bit (or longer) instructions.
		if len(data) < 4 {
			return nil, fmt.Errorf("truncated >=32-bit instruction")
		}
		instr := bo.Uint32(data)
		data = data[4:]

		_ = instr

		column := (opcode >> 2) & 0b111
		switch column {
		case 0b111:
			return nil, fmt.Errorf("extended-length instructions not supported")
		}
	}

	return result, nil
}
