//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package memory

import (
	"testing"
)

func TestMemory(t *testing.T) {
	page := Page(0)
	if page != 0 {
		t.Errorf("Page(0): %v", page)
	}
	page = Page(PageSize)
	if page != 1 {
		t.Errorf("Page(%v): %v", PageSize, page)
	}

	size := uint64(100 * PageSize)

	mem := New(RAMBase, size)
	if !mem.Contains(RAMBase) {
		t.Errorf("memory does not contain %v", RAMBase)
	}
	if mem.Contains(RAMBase + size) {
		t.Errorf("memory contains base+size %v", RAMBase+size)
	}
}
