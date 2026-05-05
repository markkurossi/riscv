//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package kernel

import (
	"os"

	"github.com/markkurossi/riscv/cpu"
)

type FD struct {
	Refcount int
	Native   *os.File
}

type Process struct {
	Kernel *Kernel
	PID    uint64
	TGID   uint64
	CPU    *cpu.CPU
	FDs    []*FD
	Ktrace bool
}

func (proc *Process) AllocFD(f *os.File) int {
	impl := &FD{
		Refcount: 1,
		Native:   f,
	}
	for fd, file := range proc.FDs {
		if file == nil {
			proc.FDs[fd] = impl
			return fd
		}
	}
	proc.FDs = append(proc.FDs, impl)
	return len(proc.FDs) - 1
}

func (proc *Process) CloseFD(fd int) bool {
	if fd < 0 || fd >= len(proc.FDs) || proc.FDs[fd] == nil {
		return false
	}
	proc.FDs[fd].Refcount--
	if proc.FDs[fd].Refcount <= 0 {
		proc.FDs[fd].Native.Close()
		proc.FDs[fd] = nil
	}
	return true
}

func (proc *Process) GetFD(fd int) *os.File {
	if fd < 0 || fd >= len(proc.FDs) || proc.FDs[fd] == nil {
		return nil
	}
	return proc.FDs[fd].Native
}

func (proc *Process) RefFD(fd int) bool {
	if fd < 0 || fd >= len(proc.FDs) || proc.FDs[fd] == nil {
		return false
	}
	proc.FDs[fd].Refcount++

	return true
}
