//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package isa

type Hart interface {
	Mode() PrivilegeMode
	Mstatus() Mstatus
	Now() uint64
	Trap(cause, tval uint64, err error) error
	ClearInterrupt(mask uint64)
	SetInterrupt(mask uint64)
	Shutdown()
	ColorOn() string
	ColorOff() string
}
