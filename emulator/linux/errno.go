//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package linux

import (
	"fmt"
)

type Errno int

const (
	ErrnoEPERM   Errno = 1
	ErrnoENOENT  Errno = 2
	ErrnoESRCH   Errno = 3
	ErrnoEINTR   Errno = 4
	ErrnoEIO     Errno = 5
	ErrnoENXIO   Errno = 6
	ErrnoE2BIG   Errno = 7
	ErrnoENOEXEC Errno = 8
	ErrnoEBADF   Errno = 9
	ErrnoECHILD  Errno = 10
	ErrnoEAGAIN  Errno = 11
	ErrnoENOMEM  Errno = 12
	ErrnoEACCES  Errno = 13
	ErrnoEFAULT  Errno = 14
	ErrnoENOTBLK Errno = 15
	ErrnoEBUSY   Errno = 16
	ErrnoEEXIST  Errno = 17
	ErrnoEXDEV   Errno = 18
	ErrnoENODEV  Errno = 19
	ErrnoENOTDIR Errno = 20
	ErrnoEISDIR  Errno = 21
	ErrnoEINVAL  Errno = 22
	ErrnoENFILE  Errno = 23
	ErrnoEMFILE  Errno = 24
	ErrnoENOTTY  Errno = 25
	ErrnoETXTBSY Errno = 26
	ErrnoEFBIG   Errno = 27
	ErrnoENOSPC  Errno = 28
	ErrnoESPIPE  Errno = 29
	ErrnoEROFS   Errno = 30
	ErrnoEMLINK  Errno = 31
	ErrnoEPIPE   Errno = 32
	ErrnoEDOM    Errno = 33
	ErrnoERANGE  Errno = 34
)

var errnos = map[Errno]string{
	ErrnoEPERM:   "Operation not permitted",
	ErrnoENOENT:  "No such file or directory",
	ErrnoESRCH:   "No such process",
	ErrnoEINTR:   "Interrupted system call",
	ErrnoEIO:     "I/O error",
	ErrnoENXIO:   "No such device or address",
	ErrnoE2BIG:   "Argument list too long",
	ErrnoENOEXEC: "Exec format error",
	ErrnoEBADF:   "Bad file number",
	ErrnoECHILD:  "No child processes",
	ErrnoEAGAIN:  "Try again",
	ErrnoENOMEM:  "Out of memory",
	ErrnoEACCES:  "Permission denied",
	ErrnoEFAULT:  "Bad address",
	ErrnoENOTBLK: "Block device required",
	ErrnoEBUSY:   "Device or resource busy",
	ErrnoEEXIST:  "File exists",
	ErrnoEXDEV:   "Cross-device link",
	ErrnoENODEV:  "No such device",
	ErrnoENOTDIR: "Not a directory",
	ErrnoEISDIR:  "Is a directory",
	ErrnoEINVAL:  "Invalid argument",
	ErrnoENFILE:  "File table overflow",
	ErrnoEMFILE:  "Too many open files",
	ErrnoENOTTY:  "Not a typewriter",
	ErrnoETXTBSY: "Text file busy",
	ErrnoEFBIG:   "File too large",
	ErrnoENOSPC:  "No space left on device",
	ErrnoESPIPE:  "Illegal seek",
	ErrnoEROFS:   "Read-only file system",
	ErrnoEMLINK:  "Too many links",
	ErrnoEPIPE:   "Broken pipe",
	ErrnoEDOM:    "Math argument out of domain of func",
	ErrnoERANGE:  "Math result not representable",
}

func (errno Errno) String() string {
	name, ok := errnos[errno]
	if ok {
		return name
	}
	return fmt.Sprintf("{Errno %d}", errno)
}
