
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
