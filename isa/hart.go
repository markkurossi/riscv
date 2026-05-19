//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package isa

type Hart interface {
	Mode() PrivilegeMode
	Mstatus() Mstatus
	Trap(cause, tval uint64, err error) error
	ColorOn()
	ColorOff()
}
