//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

// Package posix implements POSIX process abstraction.
package posix

import (
	"os"

	"github.com/markkurossi/riscv/hw"
)

type Process struct {
	CPU    *hw.CPU
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
