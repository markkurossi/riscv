//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

// Package kerlen implements the emulator kernel.
package kernel

import (
	"os"

	"github.com/markkurossi/riscv/cpu"
)

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

type Process struct {
	Kernel *Kernel
	PID    uint64
	TGID   uint64
	CPU    *cpu.CPU
	FDs    []*os.File
	Ktrace bool
}

func (proc *Process) AllocFD(f *os.File) int {
	for fd, file := range proc.FDs {
		if file == nil {
			proc.FDs[fd] = f
			return fd
		}
	}
	proc.FDs = append(proc.FDs, f)
	return len(proc.FDs) - 1
}

func (proc *Process) CloseFD(fd int) bool {
	if fd < 0 || fd >= len(proc.FDs) || proc.FDs[fd] == nil {
		return false
	}
	proc.FDs[fd].Close()
	return true
}

func (proc *Process) GetFD(fd int) *os.File {
	if fd < 0 || fd >= len(proc.FDs) {
		return nil
	}
	return proc.FDs[fd]
}
