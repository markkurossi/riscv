//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package virtio

import (
	"testing"
)

func TestEDID(t *testing.T) {
	var sum, expected byte

	//standardEDID[127] = 0x05

	for i := 0; i < 128; i++ {
		sum += standardEDID[i]
	}

	expected = 0
	for i := 0; i < 127; i++ {
		expected += standardEDID[i]
	}
	expected = byte(int(256) - int(expected))

	if sum != 0 {
		t.Errorf("EDID checksum error: checksum=0x%02x, expected=0x%02x",
			sum, expected)
	}

}
