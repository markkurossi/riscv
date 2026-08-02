//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

//go:build !amd64 && !arm64

package memory

func Uint16(buf []byte, ofs uint64) uint16 {
	return bo.Uint16(buf[ofs:])
}

func Uint32(buf []byte, ofs uint64) uint32 {
	return bo.Uint32(buf[ofs:])
}

func Uint64(buf []byte, ofs uint64) uint64 {
	return bo.Uint64(buf[ofs:])
}

func PutUint16(buf []byte, ofs uint64, v uint16) {
	bo.PutUint16(buf[ofs:], v)
}

func PutUint32(buf []byte, ofs uint64, v uint32) {
	bo.PutUint32(buf[ofs:], v)
}

func PutUint64(buf []byte, ofs, v uint64) {
	bo.PutUint64(buf[ofs:], v)
}
