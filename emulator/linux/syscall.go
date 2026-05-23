//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package linux

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/markkurossi/riscv/cpu"
	"github.com/markkurossi/riscv/isa"
	"github.com/markkurossi/riscv/kernel"
	"github.com/markkurossi/riscv/mmu"
)

var (
	bo         = binary.LittleEndian
	ErrProfile = errors.New("profile")
)

type SyscallI struct {
	Argc   int
	Format string
	Name   string
}

var SyscallInfo = map[uint64]SyscallI{
	0:   {-1, "", "io_setup"},               // 0  	| io_setup
	1:   {-1, "", "io_destroy"},             // 1  	| io_destroy
	2:   {-1, "", "io_submit"},              // 2  	| io_submit
	3:   {-1, "", "io_cancel"},              // 3  	| io_cancel
	4:   {-1, "", "io_getevents"},           // 4  	| io_getevents
	5:   {-1, "", "setxattr"},               // 5  	| setxattr
	6:   {-1, "", "lsetxattr"},              // 6  	| lsetxattr
	7:   {-1, "", "fsetxattr"},              // 7  	| fsetxattr
	8:   {-1, "", "getxattr"},               // 8  	| getxattr
	9:   {-1, "", "lgetxattr"},              // 9  	| lgetxattr
	10:  {-1, "", "fgetxattr"},              // 10 	| fgetxattr
	11:  {-1, "", "listxattr"},              // 11 	| listxattr
	12:  {-1, "", "llistxattr"},             // 12 	| llistxattr
	13:  {-1, "", "flistxattr"},             // 13 	| flistxattr
	14:  {-1, "", "removexattr"},            // 14 	| removexattr
	15:  {-1, "", "lremovexattr"},           // 15 	| lremovexattr
	16:  {-1, "", "fremovexattr"},           // 16 	| fremovexattr
	17:  {2, "pu", "getcwd"},                // 17 	| getcwd
	18:  {-1, "", "lookup_dcookie"},         // 18 	| lookup_dcookie
	19:  {-1, "", "eventfd2"},               // 19 	| eventfd2
	20:  {-1, "", "epoll_create1"},          // 20 	| epoll_create1
	21:  {-1, "", "epoll_ctl"},              // 21 	| epoll_ctl
	22:  {-1, "", "epoll_pwait"},            // 22 	| epoll_pwait
	23:  {1, "", "dup"},                     // 23 	| dup
	24:  {3, "", "dup3"},                    // 24 	| dup3
	25:  {3, "", "fcntl"},                   // 25 	| fcntl
	26:  {-1, "", "inotify_init1"},          // 26 	| inotify_init1
	27:  {-1, "", "inotify_add_watch"},      // 27 	| inotify_add_watch
	28:  {-1, "", "inotify_rm_watch"},       // 28 	| inotify_rm_watch
	29:  {3, "", "ioctl"},                   // 29 	| ioctl
	30:  {-1, "", "ioprio_set"},             // 30 	| ioprio_set
	31:  {-1, "", "ioprio_get"},             // 31 	| ioprio_get
	32:  {-1, "", "flock"},                  // 32 	| flock
	33:  {-1, "", "mknodat"},                // 33 	| mknodat
	34:  {-1, "", "mkdirat"},                // 34 	| mkdirat
	35:  {-1, "", "unlinkat"},               // 35 	| unlinkat
	36:  {-1, "", "symlinkat"},              // 36 	| symlinkat
	37:  {-1, "", "linkat"},                 // 37 	| linkat
	38:  {-1, "", "renameat"},               // 38 	| renameat
	39:  {-1, "", "umount2"},                // 39 	| umount2
	40:  {-1, "", "mount"},                  // 40 	| mount
	41:  {-1, "", "pivot_root"},             // 41 	| pivot_root
	42:  {-1, "", "nfsservctl"},             // 42 	| nfsservctl
	43:  {-1, "", "statfs"},                 // 43 	| statfs
	44:  {-1, "", "fstatfs"},                // 44 	| fstatfs
	45:  {-1, "", "truncate"},               // 45 	| truncate
	46:  {-1, "", "ftruncate"},              // 46 	| ftruncate
	47:  {-1, "", "fallocate"},              // 47 	| fallocate
	48:  {4, "ipii", "faccessat"},           // 48 	| faccessat
	49:  {-1, "", "chdir"},                  // 49 	| chdir
	50:  {-1, "", "fchdir"},                 // 50 	| fchdir
	51:  {-1, "", "chroot"},                 // 51 	| chroot
	52:  {-1, "", "fchmod"},                 // 52 	| fchmod
	53:  {-1, "", "fchmodat"},               // 53 	| fchmodat
	54:  {-1, "", "fchownat"},               // 54 	| fchownat
	55:  {-1, "", "fchown"},                 // 55 	| fchown
	56:  {4, "ipii", "openat"},              // 56 	| openat
	57:  {1, "", "close"},                   // 57 	| close
	58:  {-1, "", "vhangup"},                // 58 	| vhangup
	59:  {-1, "", "pipe2"},                  // 59 	| pipe2
	60:  {-1, "", "quotactl"},               // 60 	| quotactl
	61:  {3, "ipi", "getdents64"},           // 61 	| getdents64
	62:  {3, "", "lseek"},                   // 62 	| lseek
	63:  {3, "ipu", "read"},                 // 63 	| read
	64:  {3, "ipu", "write"},                // 64 	| write
	65:  {3, "", "readv"},                   // 65 	| readv
	66:  {3, "ipi", "writev"},               // 66 	| writev
	67:  {4, "", "pread64"},                 // 67 	| pread64
	68:  {4, "", "pwrite64"},                // 68 	| pwrite64
	69:  {4, "", "preadv"},                  // 69 	| preadv
	70:  {4, "", "pwritev"},                 // 70 	| pwritev
	71:  {4, "", "sendfile"},                // 71 	| sendfile
	72:  {-1, "", "pselect6"},               // 72 	| pselect6
	73:  {-1, "", "ppoll"},                  // 73 	| ppoll
	74:  {-1, "", "signalfd4"},              // 74 	| signalfd4
	75:  {-1, "", "vmsplice"},               // 75 	| vmsplice
	76:  {-1, "", "splice"},                 // 76 	| splice
	77:  {-1, "", "tee"},                    // 77 	| tee
	78:  {4, "ippi", "readlinkat"},          // 78 	| readlinkat
	79:  {4, "ippi", "newfstatat"},          // 79 	| newfstatat
	80:  {2, "ip", "fstat"},                 // 80 	| fstat
	81:  {-1, "", "sync"},                   // 81 	| sync
	82:  {-1, "", "fsync"},                  // 82 	| fsync
	83:  {-1, "", "fdatasync"},              // 83 	| fdatasync
	84:  {-1, "", "sync_file_range"},        // 84 	| sync_file_range
	85:  {-1, "", "timerfd_create"},         // 85 	| timerfd_create
	86:  {-1, "", "timerfd_settime"},        // 86 	| timerfd_settime
	87:  {-1, "", "timerfd_gettime"},        // 87 	| timerfd_gettime
	88:  {-1, "", "utimensat"},              // 88 	| utimensat
	89:  {-1, "", "acct"},                   // 89 	| acct
	90:  {-1, "", "capget"},                 // 90 	| capget
	91:  {-1, "", "capset"},                 // 91 	| capset
	92:  {-1, "", "personality"},            // 92 	| personality
	93:  {1, "i", "exit"},                   // 93 	| exit
	94:  {1, "i", "exit_group"},             // 94 	| exit_group
	95:  {-1, "", "waitid"},                 // 95 	| waitid
	96:  {1, "p", "set_tid_address"},        // 96  | set_tid_address
	97:  {-1, "", "unshare"},                // 97  | unshare
	98:  {6, "pipppp", "futex"},             // 98  | futex
	99:  {2, "pi", "set_robust_list"},       // 99  | set_robust_list
	100: {-1, "", "get_robust_list"},        // 100 | get_robust_list
	101: {2, "pp", "nanosleep"},             // 101 | nanosleep
	102: {-1, "", "getitimer"},              // 102 | getitimer
	103: {-1, "", "setitimer"},              // 103 | setitimer
	104: {-1, "", "kexec_load"},             // 104 | kexec_load
	105: {-1, "", "init_module"},            // 105 | init_module
	106: {-1, "", "delete_module"},          // 106 | delete_module
	107: {-1, "", "timer_create"},           // 107 | timer_create
	108: {-1, "", "timer_gettime"},          // 108 | timer_gettime
	109: {-1, "", "timer_getoverrun"},       // 109 | timer_getoverrun
	110: {-1, "", "timer_settime"},          // 110 | timer_settime
	111: {-1, "", "timer_delete"},           // 111 | timer_delete
	112: {-1, "", "clock_settime"},          // 112 | clock_settime
	113: {2, "up", "clock_gettime"},         // 113 | clock_gettime
	114: {-1, "", "clock_getres"},           // 114 | clock_getres
	115: {-1, "", "clock_nanosleep"},        // 115 | clock_nanosleep
	116: {-1, "", "syslog"},                 // 116 | syslog
	117: {-1, "", "ptrace"},                 // 117 | ptrace
	118: {-1, "", "sched_setparam"},         // 118 | sched_setparam
	119: {-1, "", "sched_setscheduler"},     // 119 | sched_setscheduler
	120: {-1, "", "sched_getscheduler"},     // 120 | sched_getscheduler
	121: {-1, "", "sched_getparam"},         // 121 | sched_getparam
	122: {-1, "", "sched_setaffinity"},      // 122 | sched_setaffinity
	123: {3, "uup", "sched_getaffinity"},    // 123 | sched_getaffinity
	124: {-1, "", "sched_yield"},            // 124 | sched_yield
	125: {-1, "", "sched_get_priority_max"}, // 125 | sched_get_priority_max
	126: {-1, "", "sched_get_priority_min"}, // 126 | sched_get_priority_min
	127: {-1, "", "sched_rr_get_interval"},  // 127 | sched_rr_get_interval
	128: {-1, "", "restart_syscall"},        // 128 | restart_syscall
	129: {-1, "", "kill"},                   // 129 | kill
	130: {-1, "", "tkill"},                  // 130 | tkill
	131: {-1, "", "tgkill"},                 // 131 | tgkill
	132: {-1, "", "sigaltstack"},            // 132 | sigaltstack
	133: {-1, "", "rt_sigsuspend"},          // 133 | rt_sigsuspend
	134: {-1, "", "rt_sigaction"},           // 134 | rt_sigaction
	135: {3, "ipp", "rt_sigprocmask"},       // 135 | rt_sigprocmask
	136: {-1, "", "rt_sigpending"},          // 136 | rt_sigpending
	137: {-1, "", "rt_sigtimedwait"},        // 137 | rt_sigtimedwait
	138: {-1, "", "rt_sigqueueinfo"},        // 138 | rt_sigqueueinfo
	139: {-1, "", "rt_sigreturn"},           // 139 | rt_sigreturn
	140: {-1, "", "setpriority"},            // 140 | setpriority
	141: {-1, "", "getpriority"},            // 141 | getpriority
	142: {-1, "", "reboot"},                 // 142 | reboot
	143: {-1, "", "setregid"},               // 143 | setregid
	144: {-1, "", "setgid"},                 // 144 | setgid
	145: {-1, "", "setreuid"},               // 145 | setreuid
	146: {-1, "", "setuid"},                 // 146 | setuid
	147: {-1, "", "setresuid"},              // 147 | setresuid
	148: {-1, "", "getresuid"},              // 148 | getresuid
	149: {-1, "", "setresgid"},              // 149 | setresgid
	150: {-1, "", "getresgid"},              // 150 | getresgid
	151: {-1, "", "setfsuid"},               // 151 | setfsuid
	152: {-1, "", "setfsgid"},               // 152 | setfsgid
	153: {-1, "", "times"},                  // 153 | times
	154: {-1, "", "setpgid"},                // 154 | setpgid
	155: {-1, "", "getpgid"},                // 155 | getpgid
	156: {-1, "", "getsid"},                 // 156 | getsid
	157: {-1, "", "setsid"},                 // 157 | setsid
	158: {-1, "", "getgroups"},              // 158 | getgroups
	159: {-1, "", "setgroups"},              // 159 | setgroups
	160: {1, "p", "uname"},                  // 160 | uname
	161: {-1, "", "sethostname"},            // 161 | sethostname
	162: {-1, "", "setdomainname"},          // 162 | setdomainname
	163: {-1, "", "getrlimit"},              // 163 | getrlimit
	164: {-1, "", "setrlimit"},              // 164 | setrlimit
	165: {-1, "", "getrusage"},              // 165 | getrusage
	166: {-1, "", "umask"},                  // 166 | umask
	167: {-1, "", "prctl"},                  // 167 | prctl
	168: {-1, "", "getcpu"},                 // 168 | getcpu
	169: {-1, "", "gettimeofday"},           // 169 | gettimeofday
	170: {-1, "", "settimeofday"},           // 170 | settimeofday
	171: {-1, "", "adjtimex"},               // 171 | adjtimex
	172: {-1, "", "getpid"},                 // 172 | getpid
	173: {-1, "", "getppid"},                // 173 | getppid
	174: {0, "", "getuid"},                  // 174 | getuid
	175: {-1, "", "geteuid"},                // 175 | geteuid
	176: {-1, "", "getgid"},                 // 176 | getgid
	177: {-1, "", "getegid"},                // 177 | getegid
	178: {-1, "", "gettid"},                 // 178 | gettid
	179: {-1, "", "sysinfo"},                // 179 | sysinfo
	180: {-1, "", "mq_open"},                // 180 | mq_open
	181: {-1, "", "mq_unlink"},              // 181 | mq_unlink
	182: {-1, "", "mq_timedsend"},           // 182 | mq_timedsend
	183: {-1, "", "mq_timedreceive"},        // 183 | mq_timedreceive
	184: {-1, "", "mq_notify"},              // 184 | mq_notify
	185: {-1, "", "mq_getsetattr"},          // 185 | mq_getsetattr
	186: {-1, "", "msgget"},                 // 186 | msgget
	187: {-1, "", "msgctl"},                 // 187 | msgctl
	188: {-1, "", "msgrcv"},                 // 188 | msgrcv
	189: {-1, "", "msgsnd"},                 // 189 | msgsnd
	190: {-1, "", "semget"},                 // 190 | semget
	191: {-1, "", "semctl"},                 // 191 | semctl
	192: {-1, "", "semtimedop"},             // 192 | semtimedop
	193: {-1, "", "semop"},                  // 193 | semop
	194: {-1, "", "shmget"},                 // 194 | shmget
	195: {-1, "", "shmctl"},                 // 195 | shmctl
	196: {-1, "", "shmat"},                  // 196 | shmat
	197: {-1, "", "shmdt"},                  // 197 | shmdt
	198: {3, "", "socket"},                  // 198 | socket
	199: {-1, "", "socketpair"},             // 199 | socketpair
	200: {3, "", "bind"},                    // 200 | bind
	201: {2, "", "listen"},                  // 201 | listen
	202: {3, "", "accept"},                  // 202 | accept
	203: {3, "", "connect"},                 // 203 | connect
	204: {-1, "", "getsockname"},            // 204 | getsockname
	205: {-1, "", "getpeername"},            // 205 | getpeername
	206: {6, "", "sendto"},                  // 206 | sendto
	207: {6, "", "recvfrom"},                // 207 | recvfrom
	208: {5, "", "setsockopt"},              // 208 | setsockopt
	209: {5, "", "getsockopt"},              // 209 | getsockopt
	210: {2, "", "shutdown"},                // 210 | shutdown
	211: {-1, "", "sendmsg"},                // 211 | sendmsg
	212: {-1, "", "recvmsg"},                // 212 | recvmsg
	213: {-1, "", "readahead"},              // 213 | readahead
	214: {1, "p", "brk"},                    // 214 | brk
	215: {2, "pi", "munmap"},                // 215 | munmap
	216: {-1, "", "mremap"},                 // 216 | mremap
	217: {-1, "", "add_key"},                // 217 | add_key
	218: {-1, "", "request_key"},            // 218 | request_key
	219: {-1, "", "keyctl"},                 // 219 | keyctl
	220: {5, "upppp", "clone"},              // 220 | clone
	221: {3, "", "execve"},                  // 221 | execve
	222: {6, "piiiii", "mmap"},              // 222 | mmap
	223: {-1, "", "fadvise64"},              // 223 | fadvise64
	224: {-1, "", "swapon"},                 // 224 | swapon
	225: {-1, "", "swapoff"},                // 225 | swapoff
	226: {3, "pui", "mprotect"},             // 226 | mprotect
	227: {3, "", "msync"},                   // 227 | msync
	228: {-1, "", "mlock"},                  // 228 | mlock
	229: {-1, "", "munlock"},                // 229 | munlock
	230: {-1, "", "mlockall"},               // 230 | mlockall
	231: {-1, "", "munlockall"},             // 231 | munlockall
	232: {-1, "", "mincore"},                // 232 | mincore
	233: {3, "pui", "madvise"},              // 233 | madvise
	234: {-1, "", "remap_file_pages"},       // 234 | remap_file_pages
	235: {-1, "", "mbind"},                  // 235 | mbind
	236: {-1, "", "get_mempolicy"},          // 236 | get_mempolicy
	237: {-1, "", "set_mempolicy"},          // 237 | set_mempolicy
	238: {-1, "", "migrate_pages"},          // 238 | migrate_pages
	239: {-1, "", "move_pages"},             // 239 | move_pages
	240: {-1, "", "rt_tgsigqueueinfo"},      // 240 | rt_tgsigqueueinfo
	241: {-1, "", "perf_event_open"},        // 241 | perf_event_open
	242: {-1, "", "accept4"},                // 242 | accept4
	243: {-1, "", "recvmmsg"},               // 243 | recvmmsg
	244: {-1, "", "wait4"},
	245: {-1, "", "prlimit64"},
	246: {-1, "", "fanotify_init"},
	247: {-1, "", "fanotify_mark"},
	248: {-1, "", "name_to_handle_at"},
	249: {-1, "", "open_by_handle_at"},
	250: {-1, "", "clock_adjtime"},
	251: {-1, "", "syncfs"},
	252: {-1, "", "sendmmsg"},
	253: {-1, "", "setns"},
	254: {-1, "", "process_vm_readv"},
	255: {-1, "", "process_vm_writev"},
	256: {-1, "", "kcmp"},
	257: {-1, "", "finit_module"},
	258: {3, "upu", "sched_setattr"},
	259: {-1, "", "sched_getattr"},
	260: {-1, "", "renameat2"},
	261: {4, "iipp", "prlimit64"},
	262: {-1, "", "getrandom"},
	263: {-1, "", "memfd_create"},
	264: {-1, "", "bpf"},
	265: {-1, "", "execveat"},
	266: {-1, "", "userfaultfd"},
	267: {-1, "", "membarrier"},
	268: {-1, "", "mlock2"},
	269: {-1, "", "copy_file_range"},
	270: {-1, "", "preadv2"},
	271: {-1, "", "pwritev2"},
	272: {-1, "", "pkey_mprotect"},
	273: {-1, "", "pkey_alloc"},
	274: {-1, "", "pkey_free"},
	275: {-1, "", "statx"},
	276: {-1, "", "io_pgetevents"},
	277: {-1, "", "rseq"},
	278: {3, "pii", "getrandom"},
	279: {-1, "", "pidfd_send_signal"},
	280: {-1, "", "io_uring_setup"},
	281: {-1, "", "io_uring_enter"},
	282: {-1, "", "io_uring_register"},
	283: {-1, "", "open_tree"},
	284: {-1, "", "move_mount"},
	285: {-1, "", "fsopen"},
	286: {-1, "", "fsconfig"},
	287: {-1, "", "fsmount"},
	288: {-1, "", "fspick"},
	289: {-1, "", "pidfd_open"},
	290: {-1, "", "clone3"},
	291: {-1, "", "close_range"},
	292: {-1, "", "openat2"},
	293: {4, "puiu", "rseq"},
	294: {-1, "", "faccessat2"},
	295: {-1, "", "process_madvise"},
	296: {-1, "", "epoll_pwait2"},
	297: {-1, "", "mount_setattr"},
	298: {-1, "", "quotactl_fd"},
	299: {-1, "", "landlock_create_ruleset"},
	300: {-1, "", "landlock_add_rule"},
	301: {-1, "", "landlock_restrict_self"},
	302: {-1, "", "memfd_secret"},
	303: {-1, "", "process_mrelease"},
	304: {-1, "", "futex_waitv"},
	305: {-1, "", "set_mempolicy_home_node"},
}

func Error(errno Errno) uint64 {
	return uint64(int64(-errno))
}

func Syscall(proc *kernel.Process, id, a0, a1, a2, a3, a4, a5 uint64) (
	uint64, error) {

	ktrace(proc, id, a0, a1, a2, a3, a4, a5)

	ret, err := linuxSyscall(proc, id, a0, a1, a2, a3, a4, a5)

	ktraceResult(proc, id, ret, err)

	return ret, err
}

func linuxSyscall(proc *kernel.Process, id, a0, a1, a2, a3, a4, a5 uint64) (
	uint64, error) {

	switch id {
	case 17: // getcwd
		cwd := "/"

		data := []byte(cwd)
		if uint64(len(data)+1) > a1 {
			data = data[:a1-1]
		}
		data = append(data, 0)
		if err := proc.MMU.CopyToUser(a0, data); err != nil {
			return Error(ErrnoEFAULT), nil
		}
		return 0, nil

	case 48: // faccessat
		dirfd := int64(a0)
		addr := a1
		mode := int64(a2)
		flags := int64(a3)

		_ = dirfd
		_ = mode
		_ = flags

		pathname, err := proc.MMU.UserCString(addr)
		if err != nil {
			return Error(ErrnoEFAULT), nil
		}
		ktracef(proc, "     pathname=%v\n", pathname)
		_, err = os.Stat(proc.Kernel.MakePath(pathname))
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

		pathname, err := proc.MMU.UserCString(addr)
		if err != nil {
			return Error(ErrnoEFAULT), nil
		}
		ktracef(proc, "     pathname=%v\n", pathname)
		f, err := os.Open(proc.Kernel.MakePath(pathname))
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
		if err := proc.MMU.CopyToUser(addr, buf[:n]); err != nil {
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
			b, err := proc.MMU.Load8(addr + i)
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
		var buf [1024]byte

		for i := 0; i < iovcnt; i++ {
			base, err := proc.MMU.Load64(iov)
			if err != nil {
				return Error(ErrnoEFAULT), nil
			}
			l, err := proc.MMU.Load64(iov + 8)
			if err != nil {
				return Error(ErrnoEFAULT), nil
			}
			iov += 16

			for l > 0 {
				n := l
				if n > uint64(len(buf)) {
					n = uint64(len(buf))
				}
				err = proc.MMU.CopyFromUser(base, buf[:n])
				if err != nil {
					return Error(ErrnoEFAULT), nil
				}
				_, err = f.Write(buf[:n])
				if err != nil {
					return Error(ErrnoEIO), nil
				}
				wrote += uint64(n)
				l -= n
				base += n
			}
		}
		return wrote, nil

	case 78: // readlinkat
		arg0 := int64(a0)
		if arg0 == AtFdcwd {
			ktracef(proc, "     - AT_FDCWD\n")
		}
		return Error(ErrnoENOENT), nil

	case 79: // newfstatat
		arg0 := int64(a0)
		statAddr := a2

		if arg0 == AtFdcwd {
			ktracef(proc, "     - AT_FDCWD\n")
		}
		pathname, err := proc.MMU.UserCString(a1)
		if err != nil {
			return Error(ErrnoEFAULT), nil
		}
		fi, err := os.Stat(pathname)
		if err != nil {
			return Error(ErrnoEIO), nil
		}
		stat := MarshalFileInfo(fi)

		if err := proc.MMU.CopyToUser(statAddr, stat); err != nil {
			return Error(ErrnoEFAULT), nil
		}
		return 0, nil

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

		stat := MarshalFileInfo(fi)

		if err := proc.MMU.CopyToUser(statAddr, stat); err != nil {
			return Error(ErrnoEFAULT), nil
		}
		return 0, nil

	case 93: // exit
		if proc.Kernel.Profile {
			return Error(ErrnoEINTR), ErrProfile
		}
		os.Exit(int(a0))

	case 94: // exit_group
		if proc.Kernel.Profile {
			return Error(ErrnoEINTR), ErrProfile
		}
		runtime := time.Since(proc.CPU.StartTime)
		runtimeS := float64(runtime) / float64(time.Second)
		ktracef(proc, "     val=%v, instret=%v, runtime=%v, MIPS=%.2f\n",
			a0, proc.CPU.Instret, runtime,
			float64(proc.CPU.Instret)/runtimeS/1000000.0)
		os.Exit(int(a0))

	case 96: // set_tid_address
		return proc.PID, nil // Caller's thread ID.

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
			v, err := proc.MMU.Load32(addr)
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
		tvSec, err := proc.MMU.Load64(a0)
		if err != nil {
			return Error(ErrnoEFAULT), nil
		}
		tvNsec, err := proc.MMU.Load64(a0 + 8)
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

		if err := proc.MMU.CopyToUser(addr, buf[:]); err != nil {
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

	case 160: // uname
		var buf [390]byte
		copy(buf[0:64], []byte("GoEMU Linux"))
		copy(buf[65:129], []byte("goemu.local"))
		copy(buf[130:194], []byte("0.0.1"))
		copy(buf[195:259], []byte("GoEMU Kernel Version 0.0"))
		copy(buf[260:], []byte("riscv"))

		if err := proc.MMU.CopyToUser(a0, buf[:]); err != nil {
			return Error(ErrnoEFAULT), nil
		}
		return 0, nil

	case 214: // brk
		if a0 > proc.Kernel.HeapEnd {
			// Compute brk.
			brk := (a0 + 4095) & ^uint64(0xfff)

			err := proc.Kernel.AddVMA(proc.Kernel.HeapEnd, brk,
				mmu.AccessRead|mmu.AccessWrite, nil, 0)
			if err != nil {
				return Error(ErrnoENOMEM), nil
			}

			if proc.Ktrace {
				ktracef(proc, "     brk: => %x - %x\n",
					proc.Kernel.HeapEnd, brk)
				ktracef(proc, "     VMA:\n")
				for i, vma := range proc.Kernel.VMA {
					ktracef(proc, "     %3d: %v\n", i, vma)
				}
			}

			proc.Kernel.HeapEnd = brk
		}
		return proc.Kernel.HeapEnd, nil

	case 215: // munmap
		// XXX check if the region was mmap'ed

	case 220: // clone
		flags := a0

		// Create child.

		// The CPU updates PC after the instruction completes. The
		// cloned process start executing with PROC.CPU.Run() so we must
		// increment PC to the next instruction. The ecall instruction
		// is always 32 bits so the +4 below works.
		child := proc.Kernel.NewProcess(proc)
		child.CPU = &cpu.CPU{
			PID:         child.PID,
			PC:          proc.CPU.PC + 4,
			MMU:         proc.CPU.MMU,
			TrapHandler: proc.CPU.TrapHandler,
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
		go func(c *kernel.Process) {
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
		fd := int(a4)
		offset := a5

		var addr uint64

		if a0 == 0 {
			// Choose address from the mmap region
			addr = proc.Kernel.MmapEnd
		} else {
			// XXX
			ktracef(proc, "     ?? using provided address %x\n", a0)
			addr = a0
		}
		var f *os.File
		if fd >= 0 {
			f = proc.GetFD(fd)
			if f == nil {
				return Error(ErrnoEBADF), nil
			}
			if offset&0xfff != 0 {
				// Offset must be multiple of page size.
				return Error(ErrnoEINVAL), nil
			}
		} else if offset != 0 {
			// No source, offset must be zero.
			return Error(ErrnoEINVAL), nil
		}

		var ps []string
		var vmaProt int

		if prot&ProtRead != 0 {
			ps = append(ps, "read")
			vmaProt |= mmu.AccessRead
		}
		if prot&ProtWrite != 0 {
			ps = append(ps, "write")
			vmaProt |= mmu.AccessWrite
		}
		if prot&ProtExec != 0 {
			ps = append(ps, "exec")
			vmaProt |= mmu.AccessExec
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
		ktracef(proc, "     fd=%v, offset=%x\n", fd, offset)

		// Align size to page size.
		length = (length + 4095) &^ 4095

		// Add a reference to the file descriptor.
		if f != nil && !proc.RefFD(fd) {
			return Error(ErrnoEBADF), nil
		}

		err := proc.Kernel.AddVMA(addr, addr+length, vmaProt, f, offset)
		if err != nil {
			return Error(ErrnoENOMEM), nil
		}

		// Update pointer for next call.
		proc.Kernel.MmapEnd += length

		ktracef(proc, "     => %x:%x\n", addr, addr+length)

		// Return the allocated address in A0
		return addr, nil

	case 226: // mprotec
		addr := a0
		size := a1
		prot := int(a2)

		if addr&0xfff != 0 || size == 0 {
			return Error(ErrnoEINVAL), nil
		}
		// Round size to full pages.
		size = (size + 4095) & 0xfff

		var p []string
		var vmaProt int
		if prot&ProtRead != 0 {
			p = append(p, "R")
			vmaProt |= mmu.AccessRead
		}
		if prot&ProtWrite != 0 {
			p = append(p, "W")
			vmaProt |= mmu.AccessWrite
		}
		if prot&ProtExec != 0 {
			p = append(p, "X")
			vmaProt |= mmu.AccessExec
		}
		ktracef(proc, "     mprotect: %x:%x: %v\n", addr, addr+size,
			strings.Join(p, ","))

		// XXX if new region overlaps and protection flags conflict,
		// we must delete corresponding TLB entries and clear page
		// table pages.

		err := proc.Kernel.AddVMA(addr, addr+size, vmaProt, nil, 0)
		if err != nil {
			fmt.Printf("EFAULT %x:%x\n", addr, addr+size)
			proc.Kernel.PrintVMA()
			return Error(ErrnoEFAULT), nil
		}

	case 261: // prlimit64

	case 278: // getrandom
		addr := a0
		len := a1
		random := make([]byte, len)
		if _, err := rand.Read(random); err != nil {
			return Error(ErrnoEFAULT), nil
		}
		if err := proc.MMU.CopyToUser(addr, random); err != nil {
			return Error(ErrnoEFAULT), nil
		}
		return len, nil

	default:
		ktracef(proc, "RET  skipping syscall %v\n", id)
	}

	return 0, nil
}

func ktraceHeader(proc *kernel.Process) {
	fmt.Printf("%5d %5d ", proc.PID, proc.TGID)
}

func ktrace(proc *kernel.Process, id, a0, a1, a2, a3, a4, a5 uint64) {
	if !proc.Ktrace {
		return
	}

	ktraceHeader(proc)

	info, ok := SyscallInfo[id]
	if !ok {
		fmt.Printf("CALL %v(%v,%v,%v,%v,%v,%v)\n", id, a0, a1, a2, a3, a4, a5)
	} else if info.Argc < 0 {
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

func ktracef(proc *kernel.Process, format string, args ...interface{}) {
	if !proc.Ktrace {
		return
	}
	ktraceHeader(proc)
	fmt.Printf(format, args...)
}

func ktraceResult(proc *kernel.Process, id, ret uint64, err error) {
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
