//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package isa

type Program struct {
	Functions []*Function
	Code      []Instr
	symbols   map[uint64]*Function
}

func NewProgram() *Program {
	return &Program{
		symbols: make(map[uint64]*Function),
	}
}

type Function struct {
	Name   string
	Addr   uint64
	Offset int
}

func (prog *Program) AddFunction(name string, addr uint64) {
	f := &Function{
		Name: name,
		Addr: addr,
	}
	prog.Functions = append(prog.Functions, f)
	prog.symbols[addr] = f
}
