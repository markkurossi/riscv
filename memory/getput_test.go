//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package memory

import (
	"encoding/binary"
	"testing"
	"unsafe"
)

func BenchmarkGetPutUnsafe(b *testing.B) {
	const count int = 1024
	buf := make([]byte, count*8)

	var max uint64

	for b.Loop() {
		sum := sumUnsafe(buf)
		if sum > max {
			max = sum
		}
	}
	_ = max
}

func sumUnsafe(buf []byte) uint64 {
	l := len(buf)
	var sum uint64
	for i := 0; i < l; i += 8 {
		sum += *(*uint64)(unsafe.Pointer(&buf[i]))
	}
	return sum
}

func BenchmarkGetPutBinary(b *testing.B) {
	const count int = 1024
	buf := make([]byte, count*8)

	var max uint64

	for b.Loop() {
		sum := sumBinary(buf)
		if sum > max {
			max = sum
		}
	}
	_ = max
}

func sumBinary(buf []byte) uint64 {
	l := len(buf)
	var sum uint64
	for i := 0; i < l; i += 8 {
		sum += binary.LittleEndian.Uint64(buf[i:])
	}
	return sum
}
