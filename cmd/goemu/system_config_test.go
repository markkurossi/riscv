//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package main

import (
	"testing"
)

var memTests = []struct {
	input string
	mem   uint64
}{
	{"512", 512},
	{"512k", 512 * 1024},
	{"512K", 512 * 1024},
	{"512m", 512 * 1024 * 1024},
	{"512M", 512 * 1024 * 1024},
	{"512g", 512 * 1024 * 1024 * 1024},
	{"512G", 512 * 1024 * 1024 * 1024},
}

func TestParseMem(t *testing.T) {
	for _, test := range memTests {
		m, err := ParseMem(test.input)
		if err != nil {
			t.Errorf("parseMem(%v) failed: %v", test.input, err)
			continue
		}
		if m != test.mem {
			t.Errorf("parseMem(%v)=%v, expected %v", test.input, m, test.mem)
		}
	}
}
