//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package isa

import (
	"testing"
)

func TestOpNameLen(t *testing.T) {
	var maxLen int
	var max []string

	var checkInfo = func(info OpInfo) {
		l := len(info.Name)
		if l > maxLen {
			maxLen = l
			max = nil
		}
		if l == maxLen {
			max = append(max, info.Name)
		}
	}

	for _, info := range Operands {
		checkInfo(info)
	}
	for _, info := range VpuOperands {
		checkInfo(info)
	}
	if maxLen != maxOpNameLen {
		t.Errorf("maxOpNameLen=%v but len(%v)=%v", maxOpNameLen, max, maxLen)
	}
}

func TestOpCount(t *testing.T) {
	if Vpu != 252 {
		t.Errorf("max op %v = %d", Vpu, int(Vpu))
	}
}
