//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

//go:build !amd64 && !arm64

package cpu

func GetUint16(buf []byte, ofs uint64) uint16 {
	return bo.Uint16(buf[ofs:])
}

func GetUint32(buf []byte, ofs uint64) uint32 {
	return bo.Uint32(buf[ofs:])
}

func GetUint64(buf []byte, ofs uint64) uint64 {
	return bo.Uint64(buf[ofs:])
}

func PutUint64(buf []byte, ofs, v uint64) {
	bo.PutUint64(buf[ofs:], v)
}
