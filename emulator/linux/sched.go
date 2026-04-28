//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package linux

import (
	"strings"
)

const (
	CSIGNAL            = 0x000000ff
	CloneVM            = 0x00000100
	CloneFS            = 0x00000200
	CloneFiles         = 0x00000400
	CloneSighand       = 0x00000800
	ClonePidfd         = 0x00001000
	ClonePtrace        = 0x00002000
	CloneVfork         = 0x00004000
	CloneParent        = 0x00008000
	CloneThread        = 0x00010000
	CloneNewns         = 0x00020000
	CloneSysvsem       = 0x00040000
	CloneSettls        = 0x00080000
	CloneParentSettid  = 0x00100000
	CloneChildCleartid = 0x00200000
	CloneDetached      = 0x00400000
	CloneUntraced      = 0x00800000
	CloneChildSettid   = 0x01000000
	CloneNewcgroup     = 0x02000000
	CloneNewuts        = 0x04000000
	CloneNewipc        = 0x08000000
	CloneNewuser       = 0x10000000
	CloneNewpid        = 0x20000000
	CloneNewnet        = 0x40000000
	CloneIO            = 0x80000000
)

func cloneFlags(flags uint64) string {
	var result []string

	if flags&CSIGNAL != 0 {
		result = append(result, "CSIGNAL")
	}
	if flags&CloneVM != 0 {
		result = append(result, "VM")
	}
	if flags&CloneFS != 0 {
		result = append(result, "FS")
	}
	if flags&CloneFiles != 0 {
		result = append(result, "FILES")
	}
	if flags&CloneSighand != 0 {
		result = append(result, "SIGHAND")
	}
	if flags&ClonePidfd != 0 {
		result = append(result, "PIDFD")
	}
	if flags&ClonePtrace != 0 {
		result = append(result, "PTRACE")
	}
	if flags&CloneVfork != 0 {
		result = append(result, "VFORK")
	}
	if flags&CloneParent != 0 {
		result = append(result, "PARENT")
	}
	if flags&CloneThread != 0 {
		result = append(result, "THREAD")
	}
	if flags&CloneNewns != 0 {
		result = append(result, "NEWNS")
	}
	if flags&CloneSysvsem != 0 {
		result = append(result, "SYSVSEM")
	}
	if flags&CloneSettls != 0 {
		result = append(result, "SETTLS")
	}
	if flags&CloneParentSettid != 0 {
		result = append(result, "SETTID")
	}
	if flags&CloneChildCleartid != 0 {
		result = append(result, "CLEARTID")
	}
	if flags&CloneDetached != 0 {
		result = append(result, "DETACHED")
	}
	if flags&CloneUntraced != 0 {
		result = append(result, "UNTRACED")
	}
	if flags&CloneChildSettid != 0 {
		result = append(result, "CHILDSETTID")
	}
	if flags&CloneNewcgroup != 0 {
		result = append(result, "NEWCGROUP")
	}
	if flags&CloneNewuts != 0 {
		result = append(result, "NEWUTS")
	}
	if flags&CloneNewipc != 0 {
		result = append(result, "NEWIPC")
	}
	if flags&CloneNewuser != 0 {
		result = append(result, "NEWUSER")
	}
	if flags&CloneNewpid != 0 {
		result = append(result, "NEWPID")
	}
	if flags&CloneNewnet != 0 {
		result = append(result, "NEWNET")
	}
	if flags&CloneIO != 0 {
		result = append(result, "IO")
	}

	return strings.Join(result, ",")
}
