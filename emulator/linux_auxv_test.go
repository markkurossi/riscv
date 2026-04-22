//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package emulator

import (
	"testing"
)

func TestAuxvTypes(t *testing.T) {
	if AtL3Cacheshape != 37 {
		t.Fatalf("AtL3Cacheshape is %v, expected 37", AtL3Cacheshape)
	}
}
