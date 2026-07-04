//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package testdata

import (
	"bytes"
	"debug/elf"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/markkurossi/riscv/cpu"
	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/memory"
)

func TestISA(t *testing.T) {
	entries, err := os.ReadDir("isa")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var success, failure int
	for _, entry := range entries {
		if runTest(t, filepath.Join("isa", entry.Name())) {
			success++
		} else {
			failure++
		}
	}
	t.Logf("%v tests, %v succeeded, %v failed", len(entries), success, failure)
}

func runTest(t *testing.T, file string) bool {
	mem := memory.New(memory.RAMBase, 0x2000000)

	hart := cpu.New(mem)
	hart.SetMode(isa.ModeM)

	htif, err := loadELF(hart, mem, file)
	if err != nil {
		t.Fatalf("%v: loadELF: %v", file, err)
	}

	hart.PC = 0x8000_0000

	err = hart.Run()
	if err != nil {
		t.Fatalf("%v: hart.Run: %v", file, err)
	}
	if htif.ExitStatus != 1 {
		t.Errorf("%v: failed assertion %v", file, htif.ExitStatus>>1)
		return false
	}

	return true
}

func loadELF(hart *cpu.CPU, mem *memory.Memory, file string) (*HTIF, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	f, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	for _, prog := range f.Progs {
		switch prog.Type {
		case elf.PT_LOAD:
			if !mem.Contains(prog.Paddr) ||
				!mem.Contains(prog.Paddr+prog.Memsz-1) {
				return nil, fmt.Errorf("prog out of range: %x...%x",
					prog.Paddr, prog.Paddr+prog.Memsz)
			}
			n, err := prog.ReadAt(mem.RAM[mem.Offset(prog.Paddr):], 0)
			if n == 0 && err != nil {
				return nil, err
			}
		}
	}
	symbols, err := f.Symbols()
	if err != nil {
		return nil, err
	}
	var toAddr, toSize, fromAddr, fromSize uint64
	for _, sym := range symbols {
		switch sym.Name {
		case "tohost":
			toAddr = sym.Value
			toSize = sym.Size
		case "fromhost":
			fromAddr = sym.Value
			fromSize = sym.Size
		}
	}
	if false {
		fmt.Printf("Symbols:\n")
		fmt.Printf(" - tohost  : %x/%x\n", toAddr, toSize)
		fmt.Printf(" - fromhost: %x/%x\n", fromAddr, fromSize)
	}

	if toAddr == 0 {
		return nil, fmt.Errorf("tohost undefined")
	}

	var start uint64
	var size uint64

	if toAddr < fromAddr {
		start = toAddr
		size = fromAddr + fromSize - toAddr
	} else {
		start = fromAddr
		size = toAddr + toSize - fromAddr
	}

	htif := NewHTIF(hart, start, size, toAddr, fromAddr, mem)

	hart.MMU.Overlay = htif

	return htif, nil
}
