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
	Start uint64
	End   uint64
	Type  rune
	Name  string
}

func (entry *SymEntry) Contains(addr uint64) bool {
	return entry.Start <= addr && addr < entry.End
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
	var last *SymEntry

	reader := bufio.NewReader(f)
	for {
		arr, err := reader.ReadBytes('\n')
		if len(arr) > 0 {
			parts := strings.Split(string(arr), " ")
			if len(parts) >= 3 {
				addr, err := strconv.ParseUint(parts[0], 16, 64)
				if err != nil {
					fmt.Printf("invalid line: %v\n", string(arr))
					continue
				}
				var t rune
				if len(parts[1]) == 0 {
					t = '?'
				} else {
					t = rune(parts[1][0])
				}
				entry := &SymEntry{
					Start: addr,
					End:   addr,
					Type:  t,
					Name:  strings.TrimSpace(parts[2]),
				}
				result.Entries = append(result.Entries, entry)
				if last != nil {
					last.End = addr
				}
				last = entry
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
		entry := sm.Entries[mid]

		if entry.Contains(addr) {
			return entry
		}

		if addr < entry.Start {
			end = mid
		} else {
			start = mid
		}
	}
}
