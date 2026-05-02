//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package linux

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/markkurossi/riscv/hw"
	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/posix"
)

var (
	bo = binary.LittleEndian
)

type SyscallI struct {
	Argc   int
	Format string
	Name   string
}

var SyscallInfo = map[uint64]SyscallI{
	0:   {0, "", "io_setup"},
	1:   {0, "", "io_destroy"},
	2:   {0, "", "io_submit"},
	3:   {0, "", "io_cancel"},
	4:   {0, "", "io_getevents"},
	5:   {0, "", "setxattr"},
	6:   {0, "", "lsetxattr"},
	7:   {0, "", "fsetxattr"},
	8:   {0, "", "getxattr"},
	9:   {0, "", "lgetxattr"},
	10:  {0, "", "fgetxattr"},
	11:  {0, "", "listxattr"},
	12:  {0, "", "llistxattr"},
	13:  {0, "", "flistxattr"},
	14:  {0, "", "removexattr"},
	15:  {0, "", "lremovexattr"},
	16:  {0, "", "fremovexattr"},
	17:  {0, "", "getcwd"},
	18:  {0, "", "lookup_dcookie"},
	19:  {0, "", "eventfd2"},
	20:  {0, "", "epoll_create1"},
	21:  {0, "", "epoll_ctl"},
	22:  {0, "", "epoll_pwait"},
	23:  {1, "", "dup"},
	24:  {3, "", "dup3"},
	25:  {3, "", "fcntl"},
	26:  {0, "", "inotify_init1"},
	27:  {0, "", "inotify_add_watch"},
	28:  {0, "", "inotify_rm_watch"},
	29:  {3, "", "ioctl"},
	30:  {0, "", "ioprio_set"},
	31:  {0, "", "ioprio_get"},
	32:  {0, "", "flock"},
	33:  {0, "", "mknodat"},
	34:  {0, "", "mkdirat"},
	35:  {0, "", "unlinkat"},
	36:  {0, "", "symlinkat"},
	37:  {0, "", "linkat"},
	38:  {0, "", "renameat"},
	39:  {0, "", "umount2"},
	40:  {0, "", "mount"},
	41:  {0, "", "pivot_root"},
	42:  {0, "", "nfsservctl"},
	43:  {0, "", "statfs"},
	44:  {0, "", "fstatfs"},
	45:  {0, "", "truncate"},
	46:  {0, "", "ftruncate"},
	47:  {0, "", "fallocate"},
	48:  {4, "ipii", "faccessat"},
	49:  {0, "", "chdir"},
	50:  {0, "", "fchdir"},
	51:  {0, "", "chroot"},
	52:  {0, "", "fchmod"},
	53:  {0, "", "fchmodat"},
	54:  {0, "", "fchownat"},
	55:  {0, "", "fchown"},
	56:  {4, "ipii", "openat"},
	57:  {1, "", "close"},
	58:  {0, "", "vhangup"},
	59:  {0, "", "pipe2"},
	60:  {0, "", "quotactl"},
	61:  {0, "", "getdents64"},
	62:  {3, "", "lseek"},
	63:  {3, "ipu", "read"},
	64:  {3, "ipu", "write"},
	65:  {3, "", "readv"},
	66:  {3, "ipi", "writev"},
	67:  {4, "", "pread64"},
	68:  {4, "", "pwrite64"},
	69:  {4, "", "preadv"},
	70:  {4, "", "pwritev"},
	71:  {4, "", "sendfile"},
	72:  {0, "", "pselect6"},
	73:  {0, "", "ppoll"},
	74:  {0, "", "signalfd4"},
	75:  {0, "", "vmsplice"},
	76:  {0, "", "splice"},
	77:  {0, "", "tee"},
	78:  {4, "ippi", "readlinkat"},
	79:  {0, "", "newfstatat"},
	80:  {2, "ip", "fstat"},
	81:  {0, "", "sync"},
	82:  {0, "", "fsync"},
	83:  {0, "", "fdatasync"},
	84:  {0, "", "sync_file_range"},
	85:  {0, "", "timerfd_create"},
	86:  {0, "", "timerfd_settime"},
	87:  {0, "", "timerfd_gettime"},
	88:  {0, "", "utimensat"},
	89:  {0, "", "acct"},
	90:  {0, "", "capget"},
	91:  {0, "", "capset"},
	92:  {0, "", "personality"},
	93:  {1, "i", "exit"},
	94:  {1, "i", "exit_group"},
	95:  {0, "", "waitid"},
	96:  {1, "p", "set_tid_address"},
	97:  {0, "", "unshare"},
	98:  {6, "pipppp", "futex"},
	99:  {2, "pi", "set_robust_list"},
	100: {0, "", "get_robust_list"},
	101: {2, "pp", "nanosleep"},
	102: {0, "", "getitimer"},
	103: {0, "", "setitimer"},
	104: {0, "", "kexec_load"},
	105: {0, "", "init_module"},
	106: {0, "", "delete_module"},
	107: {0, "", "timer_create"},
	108: {0, "", "timer_gettime"},
	109: {0, "", "timer_getoverrun"},
	110: {0, "", "timer_settime"},
	111: {0, "", "timer_delete"},
	112: {0, "", "clock_settime"},
	113: {2, "up", "clock_gettime"},
	114: {0, "", "clock_getres"},
	115: {0, "", "clock_nanosleep"},
	116: {0, "", "syslog"},
	117: {0, "", "ptrace"},
	118: {0, "", "sched_setparam"},
	119: {0, "", "sched_setscheduler"},
	120: {0, "", "sched_getscheduler"},
	121: {0, "", "sched_getparam"},
	122: {0, "", "sched_setaffinity"},
	123: {3, "uup", "sched_getaffinity"},
	124: {0, "", "sched_yield"},
	125: {0, "", "sched_get_priority_max"},
	126: {0, "", "sched_get_priority_min"},
	127: {0, "", "sched_rr_get_interval"},
	128: {0, "", "restart_syscall"},
	129: {0, "", "kill"},
	130: {0, "", "tkill"},
	131: {0, "", "tgkill"},
	132: {0, "", "sigaltstack"},
	133: {0, "", "rt_sigsuspend"},
	134: {0, "", "rt_sigaction"},
	135: {3, "ipp", "rt_sigprocmask"},
	136: {0, "", "rt_sigpending"},
	137: {0, "", "rt_sigtimedwait"},
	138: {0, "", "rt_sigqueueinfo"},
	139: {0, "", "rt_sigreturn"},
	140: {0, "", "setpriority"},
	141: {0, "", "getpriority"},
	142: {0, "", "reboot"},
	143: {0, "", "setregid"},
	144: {0, "", "setgid"},
	145: {0, "", "setreuid"},
	146: {0, "", "setuid"},
	147: {0, "", "setresuid"},
	148: {0, "", "getresuid"},
	149: {0, "", "setresgid"},
	150: {0, "", "getresgid"},
	151: {0, "", "setfsuid"},
	152: {0, "", "setfsgid"},
	153: {0, "", "times"},
	154: {0, "", "setpgid"},
	155: {0, "", "getpgid"},
	156: {0, "", "getsid"},
	157: {0, "", "setsid"},
	158: {0, "", "getgroups"},
	159: {0, "", "setgroups"},
	160: {0, "", "uname"},
	161: {0, "", "sethostname"},
	162: {0, "", "setdomainname"},
	163: {0, "", "getrlimit"},
	164: {0, "", "setrlimit"},
	165: {0, "", "getrusage"},
	166: {0, "", "umask"},
	167: {0, "", "prctl"},
	168: {0, "", "getcpu"},
	169: {0, "", "gettimeofday"},
	170: {0, "", "settimeofday"},
	171: {0, "", "adjtimex"},
	172: {0, "", "getpid"},
	173: {0, "", "getppid"},
	174: {0, "", "getuid"},
	175: {0, "", "geteuid"},
	176: {0, "", "getgid"},
	177: {0, "", "getegid"},
	178: {0, "", "gettid"},
	179: {0, "", "sysinfo"},
	180: {0, "", "mq_open"},
	181: {0, "", "mq_unlink"},
	182: {0, "", "mq_timedsend"},
	183: {0, "", "mq_timedreceive"},
	184: {0, "", "mq_notify"},
	185: {0, "", "mq_getsetattr"},
	186: {0, "", "msgget"},
	187: {0, "", "msgctl"},
	188: {0, "", "msgrcv"},
	189: {0, "", "msgsnd"},
	190: {0, "", "semget"},
	191: {0, "", "semctl"},
	192: {0, "", "semtimedop"},
	193: {0, "", "semop"},
	194: {0, "", "shmget"},
	195: {0, "", "shmctl"},
	196: {0, "", "shmat"},
	197: {0, "", "shmdt"},
	198: {3, "", "socket"},
	199: {0, "", "socketpair"},
	200: {3, "", "bind"},
	201: {2, "", "listen"},
	202: {3, "", "accept"},
	203: {3, "", "connect"},
	204: {0, "", "getsockname"},
	205: {0, "", "getpeername"},
	206: {6, "", "sendto"},
	207: {6, "", "recvfrom"},
	208: {5, "", "setsockopt"},
	209: {5, "", "getsockopt"},
	210: {2, "", "shutdown"},
	211: {0, "", "sendmsg"},
	212: {0, "", "recvmsg"},
	213: {0, "", "readahead"},
	214: {1, "p", "brk"},
	215: {2, "pi", "munmap"},
	216: {0, "", "mremap"},
	217: {0, "", "add_key"},
	218: {0, "", "request_key"},
	219: {0, "", "keyctl"},
	220: {5, "upppp", "clone"},
	221: {3, "", "execve"},
	222: {6, "piiiii", "mmap"},
	223: {0, "", "fadvise64"},
	224: {0, "", "swapon"},
	225: {0, "", "swapoff"},
	226: {3, "pui", "mprotect"},
	227: {3, "", "msync"},
	228: {0, "", "mlock"},
	229: {0, "", "munlock"},
	230: {0, "", "mlockall"},
	231: {0, "", "munlockall"},
	232: {0, "", "mincore"},
	233: {3, "pui", "madvise"},
	234: {0, "", "remap_file_pages"},
	235: {0, "", "mbind"},
	236: {0, "", "get_mempolicy"},
	237: {0, "", "set_mempolicy"},
	238: {0, "", "migrate_pages"},
	239: {0, "", "move_pages"},
	240: {0, "", "rt_tgsigqueueinfo"},
	241: {0, "", "perf_event_open"},
	242: {0, "", "accept4"},
	243: {0, "", "recvmmsg"},
	244: {0, "", "wait4"},
	245: {0, "", "prlimit64"},
	246: {0, "", "fanotify_init"},
	247: {0, "", "fanotify_mark"},
	248: {0, "", "name_to_handle_at"},
	249: {0, "", "open_by_handle_at"},
	250: {0, "", "clock_adjtime"},
	251: {0, "", "syncfs"},
	252: {0, "", "sendmmsg"},
	253: {0, "", "setns"},
	254: {0, "", "process_vm_readv"},
	255: {0, "", "process_vm_writev"},
	256: {0, "", "kcmp"},
	257: {0, "", "finit_module"},
	258: {3, "upu", "sched_setattr"},
	259: {0, "", "sched_getattr"},
	260: {0, "", "renameat2"},
	261: {4, "iipp", "prlimit64"},
	262: {0, "", "getrandom"},
	263: {0, "", "memfd_create"},
	264: {0, "", "bpf"},
	265: {0, "", "execveat"},
	266: {0, "", "userfaultfd"},
	267: {0, "", "membarrier"},
	268: {0, "", "mlock2"},
	269: {0, "", "copy_file_range"},
	270: {0, "", "preadv2"},
	271: {0, "", "pwritev2"},
	272: {0, "", "pkey_mprotect"},
	273: {0, "", "pkey_alloc"},
	274: {0, "", "pkey_free"},
	275: {0, "", "statx"},
	276: {0, "", "io_pgetevents"},
	277: {0, "", "rseq"},
	278: {3, "pii", "getrandom"},
	279: {0, "", "pidfd_send_signal"},
	280: {0, "", "io_uring_setup"},
	281: {0, "", "io_uring_enter"},
	282: {0, "", "io_uring_register"},
	283: {0, "", "open_tree"},
	284: {0, "", "move_mount"},
	285: {0, "", "fsopen"},
	286: {0, "", "fsconfig"},
	287: {0, "", "fsmount"},
	288: {0, "", "fspick"},
	289: {0, "", "pidfd_open"},
	290: {0, "", "clone3"},
	291: {0, "", "close_range"},
	292: {0, "", "openat2"},
	293: {3, "iiu", "pidfd_getfd"},
	294: {0, "", "faccessat2"},
	295: {0, "", "process_madvise"},
	296: {0, "", "epoll_pwait2"},
	297: {0, "", "mount_setattr"},
	298: {0, "", "quotactl_fd"},
	299: {0, "", "landlock_create_ruleset"},
	300: {0, "", "landlock_add_rule"},
	301: {0, "", "landlock_restrict_self"},
	302: {0, "", "memfd_secret"},
	303: {0, "", "process_mrelease"},
	304: {0, "", "futex_waitv"},
	305: {0, "", "set_mempolicy_home_node"},
}

func Error(errno Errno) uint64 {
	return uint64(int64(-errno))
}

func Syscall(proc *posix.Process, id, a0, a1, a2, a3, a4, a5 uint64) (
	uint64, error) {

	ktrace(proc, id, a0, a1, a2, a3, a4, a5)

	ret, err := syscall(proc, id, a0, a1, a2, a3, a4, a5)

	ktraceResult(proc, id, ret, err)

	return ret, err
}

var root string = "image"

func mkpath(pathname string) string {
	return filepath.Join(root, pathname)
}

func syscall(proc *posix.Process, id, a0, a1, a2, a3, a4, a5 uint64) (
	uint64, error) {

	cpu := proc.CPU

	switch id {
	case 48: // faccessat
		dirfd := int64(a0)
		addr := a1
		mode := int64(a2)
		flags := int64(a3)

		_ = dirfd
		_ = mode
		_ = flags

		pathname, err := cpu.Mem.LoadString(addr)
		if err != nil {
			return Error(ErrnoEFAULT), nil
		}
		fmt.Printf("     - pathname=%v => %v\n", pathname, mkpath(pathname))
		_, err = os.Stat(mkpath(pathname))
		if err != nil {
			return Error(ErrnoENOENT), nil
		}

		return 0, nil

	case 56: // openat
		dirfd := int64(a0)
		addr := a1
		flags := int64(a2)
		mode := int64(a3)

		_ = dirfd
		_ = mode
		_ = flags

		pathname, err := cpu.Mem.LoadString(addr)
		if err != nil {
			return Error(ErrnoEFAULT), nil
		}
		fmt.Printf("     - pathname=%v\n", pathname)
		f, err := os.Open(mkpath(pathname))
		if err != nil {
			return Error(ErrnoENOENT), nil
		}

		return uint64(proc.AllocFD(f)), nil

	case 57: // close
		if !proc.CloseFD(int(a0)) {
			return Error(ErrnoEBADF), nil
		}
		return 0, nil

	case 63: // read
		addr := a1
		length := a2

		f := proc.GetFD(int(a0))
		if f == nil {
			return Error(ErrnoEBADF), nil
		}
		buf := make([]byte, length)
		n, err := f.Read(buf)
		if err != nil {
			return Error(ErrnoEIO), nil
		}
		if err := cpu.Mem.StoreData(addr, buf[:n]); err != nil {
			return Error(ErrnoEFAULT), nil
		}
		return uint64(n), nil

	case 64: // write
		addr := a1
		length := a2

		f := proc.GetFD(int(a0))
		// XXX write to 0 should fail
		if f == nil || a0 == 0 {
			return Error(ErrnoEBADF), nil
		}

		var i, wrote uint64

		for i = 0; i < length; i++ {
			b, err := cpu.Mem.Load8(addr + i)
			if err != nil {
				return Error(ErrnoEFAULT), nil
			}
			n, err := f.Write([]byte{b})
			if err != nil {
				return Error(ErrnoEIO), nil
			}
			wrote += uint64(n)
		}
		return wrote, nil

	case 66: // writev
		iov := a1
		iovcnt := int(a2)

		f := proc.GetFD(int(a0))
		if f == nil {
			return Error(ErrnoEBADF), nil
		}

		var wrote uint64

		for i := 0; i < iovcnt; i++ {
			base, err := cpu.Mem.Load64(iov)
			if err != nil {
				return Error(ErrnoEFAULT), nil
			}
			l, err := cpu.Mem.Load64(iov + 8)
			if err != nil {
				return Error(ErrnoEFAULT), nil
			}
			iov += 16

			seg, ofs, err := cpu.Mem.Map(base, hw.AccessRead, int(l))
			if err != nil {
				return Error(ErrnoEFAULT), nil
			}

			if false {
				fmt.Printf("writev: iov=%d:\n%s",
					i, hex.Dump(seg.Data[ofs:ofs+l]))
			}

			n, err := f.Write(seg.Data[ofs : ofs+l])
			if err != nil {
				return 0, err
			}
			wrote += uint64(n)
		}
		return wrote, nil

	case 78: // readlinkat
		arg0 := int64(a0)
		if arg0 == AtFdcwd {
			fmt.Printf("     - AT_FDCWD\n")
		}
		return Error(ErrnoENOENT), nil

	case 80: // fstat
		statAddr := a1

		f := proc.GetFD(int(a0))
		if f == nil {
			return Error(ErrnoEBADF), nil
		}

		fi, err := f.Stat()
		if err != nil {
			return Error(ErrnoEIO), nil
		}

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

		modTime := uint64(fi.ModTime().Unix())
		bo.PutUint64(stat[72:], modTime)  // st_atime
		bo.PutUint64(stat[88:], modTime)  // st_mtime
		bo.PutUint64(stat[104:], modTime) // st_ctime

		if err := cpu.Mem.StoreData(statAddr, stat); err != nil {
			return Error(ErrnoEFAULT), nil
		}
		return 0, nil

	case 93: // exit
		os.Exit(int(a0))

	case 94: // exit_group
		// XXX return CPUError containing the exit status.
		os.Exit(int(a0))

	case 96: // set_tid_address
		return 1000, nil // Caller's thread ID.

	case 98: // futex
		addr := a0
		op := a1
		val := a2

		var opName string

		switch op & 127 {
		case 0:
			opName = "FUTEX_WAIT"
		case 1:
			opName = "FUTEX_WAKE"
		case 2:
			opName = "FUTEX_FD"
		case 3:
			opName = "FUTEX_REQUEUE"
		case 4:
			opName = "FUTEX_CMP_REQUEUE"
		case 5:
			opName = "FUTEX_WAKE_OP"
		case 6:
			opName = "FUTEX_LOCK_PI"
		case 7:
			opName = "FUTEX_UNLOCK_PI"
		case 8:
			opName = "FUTEX_TRYLOCK_PI"
		case 9:
			opName = "FUTEX_WAIT_BITSET"
		case 10:
			opName = "FUTEX_WAKE_BITSET"
		case 11:
			opName = "FUTEX_WAIT_REQUEUE_PI"
		case 12:
			opName = "FUTEX_CMP_REQUEUE_PI"
		case 13:
			opName = "FUTEX_LOCK_PI2"
		}

		ktracef(proc, "    => futex(%x,%v[%v],%v)\n", addr, op, opName, val)
		switch op & 127 {
		case 0: // FUTEX_WAIT
			v, err := cpu.Mem.Load32(addr)
			if err != nil {
				return Error(ErrnoEFAULT), nil
			}
			ktracef(proc, "    => val=%v, wait=%v\n", v, val)
			if uint64(v) != val {
				return Error(ErrnoEAGAIN), nil
			}
			// Single-threaded emulator: no other thread will wake us,
			// so a wait on the correct value is a deadlock. Return
			// EAGAIN so the caller retries rather than hanging.
			return Error(ErrnoEAGAIN), nil

		case 1: // FUTEX_WAKE
			// No threads are waiting in a single-threaded emulator.
			// Return 0 (number of threads woken).
			return 0, nil

		default:
			// Return 0 for all other ops rather than EINVAL, which
			// glibc treats as fatal during lock initialization.
			fmt.Printf("    => unimplemented futex op %v, returning EINVAL\n",
				op&127)
			return Error(ErrnoEINVAL), fmt.Errorf("futex op %v", op)
		}

	case 99: // set_robust_list

	case 101: // nanosleep
		tvSec, err := cpu.Mem.Load64(a0)
		if err != nil {
			return Error(ErrnoEFAULT), nil
		}
		tvNsec, err := cpu.Mem.Load64(a0 + 8)
		if err != nil {
			return Error(ErrnoEFAULT), nil
		}
		t := time.Duration(tvSec) * time.Second
		t += time.Duration(tvNsec) * time.Nanosecond

		ktracef(proc, "     nanosleep: %v,%v\n", tvSec, tvNsec)
		time.Sleep(t)

		// XXX handle rem argument.

		return 0, nil

	case 113: // clock_gettime
		addr := a1
		now := time.Now()

		var buf [16]byte
		bo.PutUint64(buf[0:], uint64(now.Unix()))
		bo.PutUint64(buf[8:], uint64(now.UnixNano()%1000000000))

		if err := cpu.Mem.StoreData(addr, buf[:]); err != nil {
			return Error(ErrnoEFAULT), nil
		}
		return 0, nil

	case 134: // rt_sigaction
		// Accept signal handler registrations but don't store them;
		// the emulator doesn't deliver signals. Return success so
		// glibc doesn't abort during initialization.
		return 0, nil

	case 135: // rt_sigprocmask
		// No signal mask to manage in a single-threaded emulator.
		return 0, nil

	case 214: // brk
		if a0 > cpu.Mem.HeapEnd {
			// Compute brk.
			brk := (a0 + 4095) & ^uint64(0xfff)

			// Get current segment.
			seg, _, err := cpu.Mem.Map(cpu.Mem.HeapStart, hw.AccessNone, 8)
			if err != nil {
				// Create memory.
				seg = &hw.Segment{
					Start: cpu.Mem.HeapStart,
					End:   brk,
					Data:  make([]byte, brk-cpu.Mem.HeapStart),
					Read:  true,
					Write: true,
				}
				cpu.Mem.Add(seg)
			} else {
				// Extend current segment.
				n := make([]byte, brk-cpu.Mem.HeapStart)
				copy(n, seg.Data)
				seg.Data = n
				seg.End = brk
			}

			cpu.Mem.HeapEnd = brk
		}
		if false {
			fmt.Printf("     brk: => %x - %x\n",
				cpu.Mem.HeapStart, cpu.Mem.HeapEnd)
		}
		return cpu.Mem.HeapEnd, nil

	case 215: // munmap
		// XXX check if the region was mmap'ed

	case 220: // clone
		flags := a0

		// Create child.

		// The CPU updates PC after the instruction completes. The
		// cloned process start executing with CPU.Run() so we must
		// increment PC to the next instruction. The ecall instruction
		// is always 32 bits so the +4 below works.
		child := proc.Kernel.NewProcess(proc)
		child.CPU = &hw.CPU{
			PID:     child.PID,
			PC:      proc.CPU.PC + 4,
			Mem:     proc.CPU.Mem,
			Syscall: proc.CPU.Syscall,
		}

		// Copy registers.
		copy(child.CPU.X[:], proc.CPU.X[:])
		copy(child.CPU.F[:], proc.CPU.F[:])

		// Init child.
		child.CPU.X[isa.A0] = 0
		child.CPU.X[isa.Sp] = a1

		if flags&CloneParentSettid != 0 {
			return 0, fmt.Errorf("clone: PARENT_SETTID")
		}
		if flags&CloneSettls != 0 {
			return 0, fmt.Errorf("clone: SETTLS")
		}
		if flags&CloneChildSettid != 0 {
			return 0, fmt.Errorf("clone: CHILD_SETTID")
		}
		ktracef(child, "clone: ret=%v, PC=%x\n",
			child.CPU.X[isa.A0], child.CPU.PC)
		go func(c *posix.Process) {
			err := c.CPU.Run()
			if err != nil {
				fmt.Printf("process %v %v: %v\n", c.PID, c.TGID, err)
			} else {
				fmt.Printf("process %v %v: exit\n", c.PID, c.TGID)
			}
		}(child)

		// Parent flow.

		// XXX flags.

		ktracef(proc, "clone: ret=%v, PC=%x\n", child.PID, proc.CPU.PC)

		return child.PID, nil

	case 222: // mmap
		length := a1
		prot := a2
		flags := a3

		_ = flags

		var addr uint64

		if a0 == 0 {
			// Choose address from the mmap region
			addr = cpu.Mem.MmapEnd
		} else {
			// XXX
			ktracef(proc, "     ?? using provided address %x\n", a0)
			addr = a0
		}
		var ps []string
		if prot&ProtRead != 0 {
			ps = append(ps, "read")
		}
		if prot&ProtWrite != 0 {
			ps = append(ps, "write")
		}
		if prot&ProtExec != 0 {
			ps = append(ps, "exec")
		}
		var fs []string
		if flags&MapFixed != 0 {
			fs = append(fs, "FIXED")
		}
		if flags&MapNoreserve != 0 {
			fs = append(fs, "NORESERVE")
		}
		if flags&MapAnonymous != 0 {
			fs = append(fs, "ANONYMOUS")
		}
		if flags&MapGrowsdown != 0 {
			fs = append(fs, "GROWSDOWN")
		}
		if flags&MapDenywrite != 0 {
			fs = append(fs, "DENYWRITE")
		}
		if flags&MapExecutable != 0 {
			fs = append(fs, "EXECUTABLE")
		}
		if flags&MapLocked != 0 {
			fs = append(fs, "LOCKED")
		}
		if flags&MapPopulate != 0 {
			fs = append(fs, "POPULATE")
		}
		if flags&MapNonblock != 0 {
			fs = append(fs, "NONBLOCK")
		}
		if flags&MapStack != 0 {
			fs = append(fs, "STACK")
		}
		if flags&MapHugetlb != 0 {
			fs = append(fs, "HUGETLB")
		}
		if flags&MapFixedNoreplace != 0 {
			fs = append(fs, "FIXED_NOREPLACE")
		}

		ktracef(proc, "     prot=%v, flags=%v\n",
			strings.Join(ps, ","), strings.Join(fs, ","))

		// Align size to page size.
		length = (length + 4095) &^ 4095

		// Create the segment
		seg := &hw.Segment{
			Start: addr,
			End:   addr + length,
			Data:  make([]byte, length),
			Read:  (prot & 1) != 0, // PROT_READ
			Write: (prot & 2) != 0, // PROT_WRITE
		}
		cpu.Mem.Add(seg)

		// Update pointer for next call.
		cpu.Mem.MmapEnd += length

		ktracef(proc, "     => %x:%x\n", addr, addr+length)

		// Return the allocated address in A0
		return addr, nil

	case 226: // mprotec
		addr := a0
		size := a1
		prot := int(a2)

		var p []string
		if prot&ProtRead != 0 {
			p = append(p, "R")
		}
		if prot&ProtWrite != 0 {
			p = append(p, "W")
		}
		if prot&ProtExec != 0 {
			p = append(p, "X")
		}
		ktracef(proc, "mprotect: %x:%x: %v\n", addr, addr+size,
			strings.Join(p, ","))
		seg, _, err := cpu.Mem.Map(addr, hw.AccessNone, int(size))
		if err != nil {
			fmt.Printf("EFAULT %x:%x\n", addr, addr+size)
			fmt.Printf("       %x:%x\n", addr&^0xfff, (addr+size)&^0xfff)
			for i, seg := range cpu.Mem.Segments {
				fmt.Printf(" - %d: %v (%x:%x)\n", i, seg,
					seg.Start&^0xfff, seg.End&^0xfff)
			}
			return Error(ErrnoEFAULT), nil
		}
		seg.Read = prot&ProtRead != 0
		seg.Write = prot&ProtWrite != 0
		seg.Exec = prot&ProtExec != 0

	case 261: // prlimit64

	case 278: // getrandom
		addr := a0
		len := a1
		random := make([]byte, len)
		if _, err := rand.Read(random); err != nil {
			return Error(ErrnoEFAULT), nil
		}
		if err := cpu.Mem.StoreData(addr, random); err != nil {
			return Error(ErrnoEFAULT), nil
		}
		return len, nil

	default:
		ktracef(proc, "RET  skipping syscall %v\n", id)
	}

	return 0, nil
}

func ktraceHeader(proc *posix.Process) {
	fmt.Printf("%5d %5d ", proc.PID, proc.TGID)
}

func ktrace(proc *posix.Process, id, a0, a1, a2, a3, a4, a5 uint64) {
	if !proc.Ktrace {
		return
	}

	ktraceHeader(proc)

	info, ok := SyscallInfo[id]
	if !ok {
		fmt.Printf("CALL %v(%v,%v,%v,%v,%v,%v)\n", id, a0, a1, a2, a3, a4, a5)
	} else if info.Argc == 0 {
		fmt.Printf("CALL %v(%v,%v,%v,%v,%v,%v)\n",
			info.Name, a0, a1, a2, a3, a4, a5)
	} else if len(info.Format) > 0 {
		fmt.Printf("CALL %s(", info.Name)
		for idx, ch := range info.Format {
			if idx > 0 {
				fmt.Print(",")
			}
			arg := proc.CPU.X[int(isa.A0)+idx]

			switch ch {
			case 'i':
				fmt.Printf("%v", int64(arg))
			case 'p':
				fmt.Printf("%x", arg)
			default:
				fmt.Printf("%v", arg)
			}
		}
		fmt.Println(")")
	} else {
		fmt.Printf("CALL %s(", info.Name)
		for i := 0; i < info.Argc; i++ {
			if i > 0 {
				fmt.Print(",")
			}
			fmt.Printf("%v", proc.CPU.X[int(isa.A0)+i])
		}
		fmt.Println(")")
	}
}

func ktracef(proc *posix.Process, format string, args ...interface{}) {
	if !proc.Ktrace {
		return
	}
	ktraceHeader(proc)
	fmt.Printf(format, args...)
}

func ktraceResult(proc *posix.Process, id, ret uint64, err error) {
	if !proc.Ktrace {
		return
	}

	ktraceHeader(proc)

	var name string
	info, ok := SyscallInfo[id]
	if !ok {
		name = fmt.Sprintf("%v", id)
	} else {
		name = info.Name
	}

	if err != nil {
		fmt.Printf("ERR  %v %v\n", name, err)
	} else if int64(ret) < 0 {
		errno := Errno(-int64(ret))
		fmt.Printf("ERR  %v %v[%v]\n", name, errno, int64(ret))
	} else {
		fmt.Printf("RET  %v %v\n", name, ret)
	}
}
