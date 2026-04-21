# RISC-V in Go

## Emulator Example

``` shell
$ cd cmd/emulator/
$ ./emulator bin/hello
   1002c:   fd5ff0ef    jal     -2c
   10000:   000105b7    lui     a1,0x10
   10004:   ff010113    addi    sp,sp,-16
   10008:   00f00613    addi    a2,zero,15
   1000c:   04858593    addi    a1,a1,72
   10010:   00100513    addi    a0,zero,1
   10014:   00113423    sd      ra,8(sp)
   10018:   020000ef    jal     20
   10038:   04000893    addi    a7,zero,64
   1003c:   00000073    ecall
ecall: write(1,65608,15)
Hello, RISC-V!
   10040:   00008067    jalr    zero,0(ra)
   1001c:   00813083    ld      ra,8(sp)
   10020:   00000513    addi    a0,zero,0
   10024:   01010113    addi    sp,sp,16
   10028:   00008067    jalr    zero,0(ra)
   10030:   05d00893    addi    a7,zero,93
   10034:   00000073    ecall
ecall: exit(0)
```

## /lib/ld-linux-riscv64-lp64d.so.1

``` shell
AT_SYSINFO_EHDR:      0x7fff99f7a000
AT_L1I_CACHESIZE:     0
AT_L1I_CACHEGEOMETRY: 0x0
AT_L1D_CACHESIZE:     0
AT_L1D_CACHEGEOMETRY: 0x0
AT_L2_CACHESIZE:      0
AT_L2_CACHEGEOMETRY:  0x0
AT_L3_CACHESIZE:      0
AT_L3_CACHEGEOMETRY:  0x0
AT_MINSIGSTKSZ:       1088
AT_HWCAP:             112d
AT_PAGESZ:            4096
AT_CLKTCK:            100
AT_PHDR:              0x55556992b040
AT_PHENT:             56
AT_PHNUM:             10
AT_BASE:              0x7fff99f7c000
AT_FLAGS:             0x0
AT_ENTRY:             0x555569937eb0
AT_UID:               0
AT_EUID:              0
AT_GID:               0
AT_EGID:              0
AT_SECURE:            0
AT_RANDOM:            0x7fffd935b5a0
AT_EXECFN:            /bin/date
AT_RSEQ_FEATURE_SIZE: 28
AT_RSEQ_ALIGN:        32
```

``` c
/* Auxiliary vector.  */

/* This vector is normally only used by the program interpreter.  The
   usual definition in an ABI supplement uses the name auxv_t.  The
   vector is not usually defined in a standard <elf.h> file, but it
   can't hurt.  We rename it to avoid conflicts.  The sizes of these
   types are an arrangement between the exec server and the program
   interpreter, so we don't fully specify them here.  */

typedef struct
{
  uint32_t a_type;              /* Entry type */
  union
    {
      uint32_t a_val;           /* Integer value */
      /* We use to have pointer elements added here.  We cannot do that,
         though, since it does not work when using 32-bit definitions
         on 64-bit platforms and vice versa.  */
    } a_un;
} Elf32_auxv_t;

typedef struct
{
  uint64_t a_type;              /* Entry type */
  union
    {
      uint64_t a_val;           /* Integer value */
      /* We use to have pointer elements added here.  We cannot do that,
         though, since it does not work when using 32-bit definitions
         on 64-bit platforms and vice versa.  */
    } a_un;
} Elf64_auxv_t;

/* Legal values for a_type (entry type).  */

#define AT_NULL         0               /* End of vector */
#define AT_IGNORE       1               /* Entry should be ignored */
#define AT_EXECFD       2               /* File descriptor of program */
#define AT_PHDR         3               /* Program headers for program */
#define AT_PHENT        4               /* Size of program header entry */
#define AT_PHNUM        5               /* Number of program headers */
#define AT_PAGESZ       6               /* System page size */
#define AT_BASE         7               /* Base address of interpreter */
#define AT_FLAGS        8               /* Flags */
#define AT_ENTRY        9               /* Entry point of program */
#define AT_NOTELF       10              /* Program is not ELF */
#define AT_UID          11              /* Real uid */
#define AT_EUID         12              /* Effective uid */
#define AT_GID          13              /* Real gid */
#define AT_EGID         14              /* Effective gid */
#define AT_CLKTCK       17              /* Frequency of times() */

/* Some more special a_type values describing the hardware.  */
#define AT_PLATFORM     15              /* String identifying platform.  */
#define AT_HWCAP        16              /* Machine-dependent hints about
                                           processor capabilities.  */

/* This entry gives some information about the FPU initialization
   performed by the kernel.  */
#define AT_FPUCW        18              /* Used FPU control word.  */

/* Cache block sizes.  */
#define AT_DCACHEBSIZE  19              /* Data cache block size.  */
#define AT_ICACHEBSIZE  20              /* Instruction cache block size.  */
#define AT_UCACHEBSIZE  21              /* Unified cache block size.  */

/* A special ignored value for PPC, used by the kernel to control the
   interpretation of the AUXV. Must be > 16.  */
#define AT_IGNOREPPC    22              /* Entry should be ignored.  */

#define AT_SECURE       23              /* Boolean, was exec setuid-like?  */

#define AT_BASE_PLATFORM 24             /* String identifying real platforms.*/

#define AT_RANDOM       25              /* Address of 16 random bytes.  */

#define AT_HWCAP2       26              /* More machine-dependent hints about
                                           processor capabilities.  */

#define AT_RSEQ_FEATURE_SIZE    27      /* rseq supported feature size.  */
#define AT_RSEQ_ALIGN   28              /* rseq allocation alignment.  */

/* More machine-dependent hints about processor capabilities.  */
#define AT_HWCAP3       29              /* extension of AT_HWCAP.  */
#define AT_HWCAP4       30              /* extension of AT_HWCAP.  */

#define AT_EXECFN       31              /* Filename of executable.  */

/* Pointer to the global system page used for system calls and other
   nice things.  */
#define AT_SYSINFO      32
#define AT_SYSINFO_EHDR 33

/* Shapes of the caches.  Bits 0-3 contains associativity; bits 4-7 contains
   log2 of line size; mask those to get cache size.  */
#define AT_L1I_CACHESHAPE       34
#define AT_L1D_CACHESHAPE       35
#define AT_L2_CACHESHAPE        36
#define AT_L3_CACHESHAPE        37

/* Shapes of the caches, with more room to describe them.
   *GEOMETRY are comprised of cache line size in bytes in the bottom 16 bits
   and the cache associativity in the next 16 bits.  */
#define AT_L1I_CACHESIZE        40
#define AT_L1I_CACHEGEOMETRY    41
#define AT_L1D_CACHESIZE        42
#define AT_L1D_CACHEGEOMETRY    43
#define AT_L2_CACHESIZE         44
#define AT_L2_CACHEGEOMETRY     45
#define AT_L3_CACHESIZE         46
#define AT_L3_CACHEGEOMETRY     47

#define AT_MINSIGSTKSZ          51 /* Stack needed for signal delivery  */
```
