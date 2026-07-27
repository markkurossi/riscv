//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

//go:build amd64 || arm64

package cpu

import (
	"unsafe"
)

func GetUint16(buf []byte, ofs uint64) uint16 {
	ptr := unsafe.Add(unsafe.Pointer(unsafe.SliceData(buf)), ofs)
	return *(*uint16)(ptr)
}

func GetUint32(buf []byte, ofs uint64) uint32 {
	ptr := unsafe.Add(unsafe.Pointer(unsafe.SliceData(buf)), ofs)
	return *(*uint32)(ptr)
}

func GetUint64(buf []byte, ofs uint64) uint64 {
	ptr := unsafe.Add(unsafe.Pointer(unsafe.SliceData(buf)), ofs)
	return *(*uint64)(ptr)
}

func PutUint64(buf []byte, ofs, v uint64) {
	ptr := unsafe.Add(unsafe.Pointer(unsafe.SliceData(buf)), ofs)
	*(*uint64)(ptr) = v
}
