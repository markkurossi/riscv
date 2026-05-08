
## stat, fstat, lstat

``` c
struct stat {
    unsigned long  st_dev;     // 0
    unsigned long  st_ino;     // 8
    unsigned int   st_mode;    // 16
    unsigned int   st_nlink;   // 20
    unsigned int   st_uid;     // 24
    unsigned int   st_gid;     // 28
    unsigned long  st_rdev;    // 32
    unsigned long  __pad1;     // 40
    long           st_size;    // 48
    int            st_blksize; // 56
    int            __pad2;     // 60
    long           st_blocks;  // 64

    long st_atime;             // 72
    unsigned long st_atime_nsec; // 80

    long st_mtime;             // 88
    unsigned long st_mtime_nsec; // 96

    long st_ctime;             // 104
    unsigned long st_ctime_nsec; // 112

    int __unused[2];           // 120–127
};
```

``` shell
sizeof(struct stat)=128
 - st_dev    : 0
 - st_ino    : 8
 - st_mode   : 16
 - st_nlink  : 20
 - st_uid    : 24
 - st_gid    : 28
 - st_rdev   : 32
 - st_size   : 48
 - st_blksize: 56
 - st_blocks : 64
 - st_atime  : 72
 - st_mtime  : 88
 - st_ctime  : 104
```

``` c
struct timespec {
    time_t   tv_sec;        /* seconds */
    long     tv_nsec;       /* nanoseconds */
};
```

# syscalls, the listings deviate from 244

| Nr  | Name                         |
| --- | ---------------------------- |
| 160 | uname                        |
| 161 | sethostname                  |
| 162 | setdomainname                |
| 163 | getrlimit                    |
| 164 | setrlimit                    |
| 165 | getrusage                    |
| 166 | umask                        |
| 167 | prctl                        |
| 168 | getcpu                       |
| 169 | gettimeofday                 |
| 170 | settimeofday                 |
| 171 | adjtimex                     |
| 172 | getpid                       |
| 173 | getppid                      |
| 174 | getuid                       |
| 175 | geteuid                      |
| 176 | getgid                       |
| 177 | getegid                      |
| 178 | gettid                       |
| 179 | sysinfo                      |
| 180 | mq_open                      |
| 181 | mq_unlink                    |
| 182 | mq_timedsend                 |
| 183 | mq_timedreceive              |
| 184 | mq_notify                    |
| 185 | mq_getsetattr                |
| 186 | msgget                       |
| 187 | msgctl                       |
| 188 | msgrcv                       |
| 189 | msgsnd                       |
| 190 | semget                       |
| 191 | semctl                       |
| 192 | semtimedop                   |
| 193 | semop                        |
| 194 | shmget                       |
| 195 | shmctl                       |
| 196 | shmat                        |
| 197 | shmdt                        |
| 198 | socket                       |
| 199 | socketpair                   |
| 200 | bind                         |
| 201 | listen                       |
| 202 | accept                       |
| 203 | connect                      |
| 204 | getsockname                  |
| 205 | getpeername                  |
| 206 | sendto                       |
| 207 | recvfrom                     |
| 208 | setsockopt                   |
| 209 | getsockopt                   |
| 210 | shutdown                     |
| 211 | sendmsg                      |
| 212 | recvmsg                      |
| 213 | readahead                    |
| 214 | brk                          |
| 215 | munmap                       |
| 216 | mremap                       |
| 217 | add_key                      |
| 218 | request_key                  |
| 219 | keyctl                       |
| 220 | clone                        |
| 221 | execve                       |
| 222 | mmap                         |
| 223 | fadvise64                    |
| 224 | swapon                       |
| 225 | swapoff                      |
| 226 | mprotect                     |
| 227 | msync                        |
| 228 | mlock                        |
| 229 | munlock                      |
| 230 | mlockall                     |
| 231 | munlockall                   |
| 232 | mincore                      |
| 233 | madvise                      |
| 234 | remap_file_pages             |
| 235 | mbind                        |
| 236 | get_mempolicy                |
| 237 | set_mempolicy                |
| 238 | migrate_pages                |
| 239 | move_pages                   |
| 240 | rt_tgsigqueueinfo            |
| 241 | perf_event_open              |
| 242 | accept4                      |
| 243 | recvmmsg                     |
| 244 | arch_prctl (unused on riscv) |
| 245 | wait4                        |
| 246 | prlimit64                    |
| 247 | fanotify_init                |
| 248 | fanotify_mark                |
| 249 | name_to_handle_at            |
| 250 | open_by_handle_at            |
| 251 | clock_adjtime                |
| 252 | syncfs                       |
| 253 | setns                        |
| 254 | sendmmsg                     |
| 255 | process_vm_readv             |
| 256 | process_vm_writev            |
| 257 | kcmp                         |
| 258 | finit_module                 |
| 259 | sched_setattr                |
| 260 | sched_getattr                |
| 261 | renameat2                    |
| 262 | seccomp                      |
| 263 | getrandom                    |
| 264 | memfd_create                 |
| 265 | bpf                          |
| 266 | execveat                     |
| 267 | userfaultfd                  |
| 268 | membarrier                   |
| 269 | mlock2                       |
| 270 | copy_file_range              |
| 271 | preadv2                      |
| 272 | pwritev2                     |
| 273 | pkey_mprotect                |
| 274 | pkey_alloc                   |
| 275 | pkey_free                    |
| 276 | statx                        |
| 277 | io_pgetevents                |
| 278 | rseq                         |

| Nr  | Name                    |
| --- | ----------------------- |
| 279 | kexec_file_load         |
| 280 | pidfd_send_signal       |
| 281 | io_uring_setup          |
| 282 | io_uring_enter          |
| 283 | io_uring_register       |
| 284 | open_tree               |
| 285 | move_mount              |
| 286 | fsopen                  |
| 287 | fsconfig                |
| 288 | fsmount                 |
| 289 | fspick                  |
| 290 | pidfd_open              |
| 291 | clone3                  |
| 292 | close_range             |
| 293 | openat2                 |
| 294 | pidfd_getfd             |
| 295 | faccessat2              |
| 296 | process_madvise         |
| 297 | epoll_pwait2            |
| 298 | mount_setattr           |
| 299 | quotactl_fd             |
| 300 | landlock_create_ruleset |
| 301 | landlock_add_rule       |
| 302 | landlock_restrict_self  |
| 303 | memfd_secret            |
