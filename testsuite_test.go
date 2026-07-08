//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package riscv

import (
	"bytes"
	"debug/elf"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/markkurossi/riscv/cpu"
	"github.com/markkurossi/riscv/dev"
	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/memory"
)

var skip = map[string]bool{
	"Makefile":                         true,
	"hypervisor-p-2-stage_translation": true,
	"hypervisor-p-2-stage_translation_implicit_load_error":           true,
	"hypervisor-p-2-stage_translation_implicit_load_error_hs":        true,
	"hypervisor-svadu-p-2-stage_translation_implicit_store_error":    true,
	"hypervisor-svadu-p-2-stage_translation_implicit_store_error_hs": true,
	"rv64si-p-dirty":       true,
	"rv64ssvnapot-p-napot": true,
}

func TestISA(t *testing.T) {
	dir := "testdata/isa"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var count, success, failure, panic int
	for _, entry := range entries {
		name := entry.Name()
		file := filepath.Join(dir, name)
		if strings.HasSuffix(file, ".dump") {
			continue
		}
		if skip[name] {
			continue
		}
		count++
		ok, err := runTest(t, file)
		if err != nil {
			t.Logf("%v: %v", entry.Name(), err)
			panic++
		} else if ok {
			success++
		} else {
			failure++
		}
	}
	t.Logf("%v tests, %v success, %v fail, %v panic",
		count, success, failure, panic)
}

func runTest(t *testing.T, file string) (bool, error) {
	mem := memory.New(memory.RAMBase, 0x2000000)

	hart := cpu.New(mem)
	hart.SetMode(isa.ModeM)

	htif, entry, err := loadELF(hart, mem, file)
	if err != nil {
		return false, err
	}

	hart.PC = entry

	err = hart.Run()
	if err != nil {
		return false, err
	}
	if htif.ExitStatus != 1 {
		t.Errorf("%v: assertion %v", file, htif.ExitStatus>>1)
		return false, nil
	}

	return true, nil
}

func loadELF(hart *cpu.CPU, mem *memory.Memory, file string) (
	*dev.HTIF, uint64, error) {

	data, err := os.ReadFile(file)
	if err != nil {
		return nil, 0, err
	}
	f, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	for _, prog := range f.Progs {
		switch prog.Type {
		case elf.PT_LOAD:
			if !mem.Contains(prog.Paddr) ||
				!mem.Contains(prog.Paddr+prog.Memsz-1) {
				return nil, 0, fmt.Errorf("prog out of range: %x...%x",
					prog.Paddr, prog.Paddr+prog.Memsz)
			}
			n, err := prog.ReadAt(mem.RAM[mem.Offset(prog.Paddr):], 0)
			if n == 0 && err != nil {
				return nil, 0, err
			}
		}
	}
	symbols, err := f.Symbols()
	if err != nil {
		return nil, 0, err
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
		fmt.Printf("Entry: %x\n", f.Entry)
		fmt.Printf("Symbols:\n")
		fmt.Printf(" - tohost  : %x/%x\n", toAddr, toSize)
		fmt.Printf(" - fromhost: %x/%x\n", fromAddr, fromSize)
	}

	if toAddr == 0 {
		return nil, 0, fmt.Errorf("tohost undefined")
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

	htif := dev.NewHTIF(hart, start, size, toAddr, fromAddr, mem)

	hart.MMU.Overlay = htif

	return htif, f.Entry, nil
}
