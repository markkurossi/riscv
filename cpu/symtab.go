//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package cpu

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type Symtab interface {
	Resolve(addr uint64) *SymEntry
}

var (
	_ Symtab = &SystemMap{}
)

type SymEntry struct {
	Addr uint64
	Type rune
	Name string
}

type SystemMap struct {
	Entries []*SymEntry
}

func LoadSystemMap(file string) (*SystemMap, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	result := new(SystemMap)

	reader := bufio.NewReader(f)
	for {
		arr, err := reader.ReadBytes('\n')
		if arr != nil && len(arr) > 0 {
			parts := strings.Split(string(arr), " ")
			if len(parts) >= 3 {
				addr, err := strconv.ParseUint(parts[0], 16, 64)
				if err != nil {
					fmt.Printf("invalid line: %v\n", string(arr))
					continue
				}
				var t rune
				if len(parts[1]) < 0 {
					t = '?'
				} else {
					t = rune(parts[1][0])
				}
				result.Entries = append(result.Entries, &SymEntry{
					Addr: addr,
					Type: t,
					Name: strings.TrimSpace(parts[2]),
				})
			}
		} else {
			if err == io.EOF {
				break
			}
			return nil, err
		}
	}
	return result, nil
}

func (sm *SystemMap) Resolve(addr uint64) *SymEntry {
	start := 0
	end := len(sm.Entries)

	for {
		if start+1 >= end {
			return sm.Entries[start]
		}
		mid := (end-start)/2 + start
		midAddr := sm.Entries[mid].Addr

		if addr < midAddr {
			end = mid
		} else if addr == midAddr || mid+1 >= end ||
			addr < sm.Entries[mid+1].Addr {
			return sm.Entries[mid]
		} else {
			start = mid
		}
	}
}
