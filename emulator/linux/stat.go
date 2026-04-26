//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package linux

// Constants for the *at(2) family of syscalls.
const (
	AtFdcwd = -100
)

type Stat struct {
	StDev       uint64
	StIno       uint64
	StMode      uint32
	StNlink     uint32
	StUID       uint32
	StGID       uint32
	StRdev      uint64
	Pad1        uint64
	StSize      int64
	StBlksize   int32
	Pad2        int32
	StBlocks    int64
	StAtime     int64
	StAtimeNsec uint64
	StMtime     int64
	StMtimeNsec uint64
	StCtime     int64
	StCtimeNsec uint64
	Unused      uint64
}
