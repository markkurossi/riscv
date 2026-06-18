//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package isa

// Hart defines RISC-V hardware thread interface.
type Hart interface {
	// Mode returns the current privilege mode.
	Mode() PrivilegeMode

	// Mstatus returns the machine status register.
	Mstatus() Mstatus

	// Now returns the monotonically increasing CPU time.
	Now() uint64

	// Trap creates a new exception.
	Trap(cause, tval uint64, err error) error

	// ClearInterrupt clears the specified interrupts.
	ClearInterrupt(mask uint64)

	// SetInterrupt sets the specified interrupts.
	SetInterrupt(mask uint64)

	// Shutdown initiates hart shutdown.
	Shutdown()

	// SetTrace controls the debug trace.
	SetTrace(on bool)

	// ColorOn turns on CPU's mode specific color logging.
	ColorOn() string

	// ColorOff turns off color logging.
	ColorOff() string
}
