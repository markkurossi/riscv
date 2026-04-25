//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package linux

import (
	"crypto/rand"
	"fmt"
	"os"

	"github.com/markkurossi/riscv/hw"
	"github.com/markkurossi/riscv/isa"
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
	101: {2, "", "nanosleep"},
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
	113: {0, "", "clock_gettime"},
	114: {0, "", "clock_getres"},
	115: {0, "", "clock_nanosleep"},
	116: {0, "", "syslog"},
	117: {0, "", "ptrace"},
	118: {0, "", "sched_setparam"},
	119: {0, "", "sched_setscheduler"},
	120: {0, "", "sched_getscheduler"},
	121: {0, "", "sched_getparam"},
	122: {0, "", "sched_setaffinity"},
	123: {0, "", "sched_getaffinity"},
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
	135: {0, "", "rt_sigprocmask"},
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
	220: {5, "", "clone"},
	221: {3, "", "execve"},
	222: {6, "piiiii", "mmap"},
	223: {0, "", "fadvise64"},
	224: {0, "", "swapon"},
	225: {0, "", "swapoff"},
	226: {3, "", "mprotect"},
	227: {3, "", "msync"},
	228: {0, "", "mlock"},
	229: {0, "", "munlock"},
	230: {0, "", "mlockall"},
	231: {0, "", "munlockall"},
	232: {0, "", "mincore"},
	233: {0, "", "madvise"},
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
	258: {0, "", "sched_setattr"},
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
	293: {0, "", "pidfd_getfd"},
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

func Syscall(cpu *hw.CPU, id, a0, a1, a2, a3, a4, a5 uint64) (uint64, error) {

	ktrace(cpu, id, a0, a1, a2, a3, a4, a5)

	ret, err := syscall(cpu, id, a0, a1, a2, a3, a4, a5)

	ktraceResult(cpu, id, ret, err)

	return ret, err
}

func syscall(cpu *hw.CPU, id, a0, a1, a2, a3, a4, a5 uint64) (uint64, error) {
	switch id {
	case 64: // write
		fd := a0
		addr := a1
		len := a2

		_ = fd

		var i uint64

		for i = 0; i < len; i++ {
			b, err := cpu.Mem.Load8(addr + i)
			if err != nil {
				return Error(ErrnoEFAULT), nil
			}
			os.Stdout.Write([]byte{b})
			if err != nil {
				break
			}
		}
		if i < len {
			return Error(ErrnoEIO), nil
		}
		return len, nil

	case 66: // writev
		fd := int(a0)
		iov := a1
		iovcnt := int(a2)

		var f *os.File
		switch fd {
		case 0:
			f = os.Stdin
		case 1:
			f = os.Stdout
		case 2:
			f = os.Stderr
		default:
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

			seg, ofs, err := cpu.Mem.Map(base, int(l))
			if err != nil {
				return Error(ErrnoEFAULT), nil
			}

			n, err := f.Write(seg.Data[ofs : ofs+l])
			if err != nil {
				return 0, err
			}
			wrote += uint64(n)
		}
		return wrote, nil

	case 78: // readlinkat
		const AtFdcwd int64 = -100
		arg0 := int64(a0)
		if arg0 == AtFdcwd {
			fmt.Printf("     - AT_FDCWD\n")
		}
		return Error(ErrnoENOENT), nil

	case 80: // fstat
		fd := int(a0)
		statAddr := a1

		// Only handle the standard streams; everything else is unknown.
		if fd != 0 && fd != 1 && fd != 2 {
			return Error(ErrnoEBADF), nil
		}

		// Write a minimal stat struct (riscv64 Linux layout, 128
		// bytes).  glibc reads st_mode to determine if stdout is a
		// tty (line-buffered) or a regular file (fully-buffered), and
		// st_blksize for buffer size.
		stat := make([]byte, 128)

		// st_mode @ offset 16: S_IFCHR | 0620 (character device, rw)
		mode := uint32(0020620)
		stat[16] = byte(mode)
		stat[17] = byte(mode >> 8)
		stat[18] = byte(mode >> 16)
		stat[19] = byte(mode >> 24)

		// st_nlink @ offset 20
		stat[20] = 1

		// st_uid @ offset 24: 1000
		stat[24] = 0xe8
		stat[25] = 0x03

		// st_gid @ offset 28: 1000
		stat[28] = 0xe8
		stat[29] = 0x03

		// st_rdev @ offset 40: tty device
		stat[40] = 0x88
		stat[41] = 0x08

		// st_blksize @ offset 56: 1024
		stat[56] = 0x00
		stat[57] = 0x04

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
		return 1000, nil // Caller's tread ID.

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

		fmt.Printf("    => futex(%x,%v[%v],%v)\n", addr, op, opName, val)
		switch op & 127 {
		case 0: // FUTEX_WAIT
			v, err := cpu.Mem.Load32(addr)
			if err != nil {
				return Error(ErrnoEFAULT), nil
			}
			fmt.Printf("    => val=%v, wait=%v\n", v, val)
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
			fmt.Printf("    => unimplemented futex op %v, returning 0\n",
				op&127)
			return 0, nil
		}

	case 99: // set_robust_list

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
			seg, _, err := cpu.Mem.Map(cpu.Mem.HeapStart, 8)
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

	case 222: // mmap
		length := a1
		prot := a2
		flags := a3

		_ = flags

		if a0 == 0 {
			// Choose address from the mmap region
			addr := cpu.Mem.MmapEnd

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

			fmt.Printf("    => %x:%x\n", addr, addr+length)

			// Return the allocated address in A0
			return addr, nil
		} else {
			if true {
				return 0, fmt.Errorf("mmap: unsupported flow")
			}
			return Error(ErrnoEINVAL), nil
		}

	case 226: // mprotec

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
		if cpu.Ktrace {
			fmt.Printf("RET  skipping syscall %v\n", id)
		}
	}

	return 0, nil
}

func ktrace(cpu *hw.CPU, id, a0, a1, a2, a3, a4, a5 uint64) {
	if !cpu.Ktrace {
		return
	}

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
			arg := cpu.X[int(isa.A0)+idx]

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
			fmt.Printf("%v", cpu.X[int(isa.A0)+i])
		}
		fmt.Println(")")
	}
}

func ktraceResult(cpu *hw.CPU, id, ret uint64, err error) {
	if !cpu.Ktrace {
		return
	}

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
		errno := Errno(int64(ret))
		fmt.Printf("ERR  %v %v[%v]", name, errno, int64(ret))
	} else {
		fmt.Printf("RET  %v %v\n", name, ret)
	}
}
