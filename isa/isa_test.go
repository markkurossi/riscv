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

	for _, info := range Operands {
		l := len(info.Name)
		if l > maxLen {
			maxLen = l
			max = nil
		}
		if l == maxLen {
			max = append(max, info.Name)
		}
	}
	if maxLen != maxOpNameLen {
		t.Errorf("maxOpNameLen=%v but len(%v)=%v", maxOpNameLen, max, maxLen)
	}
}

func TestOpCount(t *testing.T) {
	if Rorw != 164 {
		t.Errorf("max op Rorw = %d", int(Rorw))
	}
}
