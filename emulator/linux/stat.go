//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

//lint:file-ignore ST1003 to match the C coding style for constants.

package linux

import (
	"io/fs"
	"syscall"
)

// Constants for the *at(2) family of syscalls.
const (
	AtFdcwd = -100
)

const (
	S_IFMT   = 0170000 /* type of file */
	S_IFIFO  = 0010000 /* named pipe (fifo) */
	S_IFCHR  = 0020000 /* character special */
	S_IFDIR  = 0040000 /* directory */
	S_IFBLK  = 0060000 /* block special */
	S_IFREG  = 0100000 /* regular */
	S_IFLNK  = 0120000 /* symbolic link */
	S_IFSOCK = 0140000 /* socket */
	S_IFWHT  = 0160000 /* whiteout */

	S_ISUID = 0004000 /* set user id on execution */
	S_ISGID = 0002000 /* set group id on execution */
	S_ISVTX = 0001000 /* save swapped text even after use */
	S_IRUSR = 0000400 /* read permission, owner */
	S_IWUSR = 0000200 /* write permission, owner */
	S_IXUSR = 0000100 /* execute/search permission, owner */
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

func MarshalFileInfo(fi fs.FileInfo) []byte {
	// XXX change to use marshal(Stat)
	stat := make([]byte, 128)

	mode := fi.Mode()
	stMode := int(mode & fs.ModePerm)

	if mode&fs.ModeNamedPipe != 0 {
		stMode |= S_IFIFO
	}
	if mode&fs.ModeCharDevice != 0 {
		stMode |= S_IFCHR
	}
	if mode&fs.ModeDir != 0 {
		stMode |= S_IFDIR
	}
	if mode&fs.ModeDevice != 0 {
		stMode |= S_IFBLK
	}
	if mode&fs.ModeSymlink != 0 {
		stMode |= S_IFLNK
	}
	if mode&fs.ModeSocket != 0 {
		stMode |= S_IFSOCK
	}

	if mode&fs.ModeType == 0 {
		stMode |= S_IFREG
	}

	// st_mode @ offset 16
	bo.PutUint32(stat[16:], uint32(stMode))

	// st_nlink @ offset 20
	bo.PutUint32(stat[20:], 1)

	// st_uid @ offset 24: 1000
	bo.PutUint32(stat[24:], 1000)

	// st_gid @ offset 28: 1000
	bo.PutUint32(stat[28:], 1000)

	if mode&fs.ModeDevice != 0 {
		// st_rdev @ offset 32: tty device
		bo.PutUint64(stat[32:], 34816)
	}

	// st_size @ offset 48
	bo.PutUint64(stat[48:], uint64(fi.Size()))

	// st_blksize @ offset 56: 1024
	bo.PutUint64(stat[56:], 1024)

	// st_blocks @ offset 64: fi.Size+1023/1024
	bo.PutUint64(stat[64:], uint64(fi.Size()+1023)/1024)

	modTime := uint64(fi.ModTime().Unix())
	bo.PutUint64(stat[72:], modTime)  // st_atime
	bo.PutUint64(stat[88:], modTime)  // st_mtime
	bo.PutUint64(stat[104:], modTime) // st_ctime

	native, ok := fi.Sys().(*syscall.Stat_t)
	if ok {
		// fmt.Printf("native: %#vn", native)
		// st_ino @ offset 8
		bo.PutUint64(stat[8:], native.Ino)
	}

	return stat
}
