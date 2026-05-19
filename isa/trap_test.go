//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package isa

import (
	"testing"
)

func TestMstatus(t *testing.T) {
	var ms Mstatus

	if ms.SIE() {
		t.Error("SIE set on empty Mstatus")
	}
	ms.SetSIE(true)
	if !ms.SIE() {
		t.Error("SetSIE(true) failed")
	}
	ms.SetSIE(false)
	if ms.SIE() {
		t.Error("SetSIE(false) failed")
	}

	if ms.SPP() != ModeU {
		t.Error("SPP not ModeU")
	}
	ms.SetSPP(ModeS)
	if ms.SPP() != ModeS {
		t.Error("SetSPP(ModeS) failed")
	}

	if ms.MPP() != ModeU {
		t.Error("MPP not ModeU")
	}
	for i := PrivilegeMode(0); i <= ModeM; i++ {
		ms.SetMPP(i)
		if ms.MPP() != i {
			t.Errorf("SetMPP(%v) failed", i)
		}
	}
}
