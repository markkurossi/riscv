//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

// Package kernel implements the emulator kernel.
package kernel

type Kernel struct {
	NextPID uint64
	Ktrace  bool
}

func (kern *Kernel) NewProcess(tg *Process) *Process {
	proc := &Process{
		Kernel: kern,
		PID:    kern.NextPID,
		Ktrace: kern.Ktrace,
	}
	kern.NextPID++

	if tg != nil {
		proc.TGID = tg.TGID
	} else {
		proc.TGID = proc.PID
	}

	return proc
}
