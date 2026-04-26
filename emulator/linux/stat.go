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
