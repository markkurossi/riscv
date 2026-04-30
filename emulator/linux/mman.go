//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package linux

const (
	ProtNone  = 0
	ProtRead  = 1
	ProtWrite = 2
	ProtExec  = 4
)

const (
	MapType           = 0x00f
	MapFixed          = 0x010
	MapNoreserve      = 0x0400
	MapAnonymous      = 0x0800
	MapGrowsdown      = 0x1000
	MapDenywrite      = 0x2000
	MapExecutable     = 0x4000
	MapLocked         = 0x8000
	MapPopulate       = 0x10000
	MapNonblock       = 0x20000
	MapStack          = 0x40000
	MapHugetlb        = 0x80000
	MapFixedNoreplace = 0x100000
)
