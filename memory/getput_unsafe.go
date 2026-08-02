//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

//go:build amd64 || arm64

package memory

import (
	"unsafe"
)

func Uint16(buf []byte, ofs uint64) uint16 {
	ptr := unsafe.Add(unsafe.Pointer(unsafe.SliceData(buf)), ofs)
	return *(*uint16)(ptr)
}

func Uint32(buf []byte, ofs uint64) uint32 {
	ptr := unsafe.Add(unsafe.Pointer(unsafe.SliceData(buf)), ofs)
	return *(*uint32)(ptr)
}

func Uint64(buf []byte, ofs uint64) uint64 {
	ptr := unsafe.Add(unsafe.Pointer(unsafe.SliceData(buf)), ofs)
	return *(*uint64)(ptr)
}

func PutUint16(buf []byte, ofs uint64, v uint16) {
	ptr := unsafe.Add(unsafe.Pointer(unsafe.SliceData(buf)), ofs)
	*(*uint16)(ptr) = v
}

func PutUint32(buf []byte, ofs uint64, v uint32) {
	ptr := unsafe.Add(unsafe.Pointer(unsafe.SliceData(buf)), ofs)
	*(*uint32)(ptr) = v
}

func PutUint64(buf []byte, ofs, v uint64) {
	ptr := unsafe.Add(unsafe.Pointer(unsafe.SliceData(buf)), ofs)
	*(*uint64)(ptr) = v
}
