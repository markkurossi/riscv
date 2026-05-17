# RISC-V in Go

<img align="center" src="goemu-small.png">

A RISC-V emulator written in Go. The goal is to implement the RV64GC
(64-bit, general-purpose, compressed) profile, with support for running
Linux applications and, eventually, the Linux operating system.

TODO:

 - [x] Verify DTB (dtbtool etc.)
   - [x] CPU.Dump() at mret: is dtb passed to Linux?
   - [x] Check DTB format with dtbtool
 - [x] Walk page table and check what is at pc=ffffffff80026824
   - [x] Check Linux sources
 - [ ] Draw architecture diagram of traps
 - [ ] Study interrupts
 - [x] Interrupt check in loop
 - [ ] Create CLINT device in CPU and fix ROM to write to right fields
 - [ ] Does Image have a PE header? Does it also contain System.map?

## MMU Refactoring

 - [x] Fix page table MMU code to run the existing samples
 - [ ] Refactor the entire emulator stack
   - [ ] CPU -> Kernel -> Process -> Emulator
   - [ ] Kernel creates processes 1-n
   - [ ] Virtual memory handled per process
   - [ ] CPU has only {Load,Store}Uint{8,16,32,64}()
   - [ ] Process has {Put,}Uint{8,16,32,64,String,Data}() functions
   - [ ] Syscall can cause page fault on mmap'ed files. Syscall's
         traps should fill page table.
 - [x] sfence.vma must clear MMU's TLB entries
 - [x] Remove `Raw uint32` from `Instr`?

## Step 1 - Basics

Run statically and dynamically linked simple C applications. Provide
basic support for simple Go programs.

 - [x] Compressed instructions
 - [x] Support for most common 64-bit instructions
 - [x] Run standalone binaries
 - [x] Run statically linked, single-threaded binaries
 - [x] Run dynamically linked, single-threaded binaries

## Step 2 - MMU and Linux syscalls

 - [x] MMU with page tables
   - [ ] Move pagetable to processes
 - [ ] Refactor emulator source files
 - [ ] Rethink FD handling.
 - [ ] Run most Linux and Go binaries
 - [ ] Proper Linux syscall support

## Step 3 - Supervisor mode

 - [ ] Supervisor mode
 - [ ] Boot Linux kernel

# Benchmarks

These benchmarks are a work-in-progress performance tracker. The
numbers below are from running [fibo.c](tests/static/fibo.c) on:

``` text
cpu: Intel(R) Core(TM) i5-8257U CPU @ 1.40GHz
```

| Optimization           | fib 30 | fib 35 |    fib 40 |   MIPS | Relative |
|:-----------------------|-------:|-------:|----------:|-------:|---------:|
| Base                   |  2.969 | 32.510 |           |  19.75 |    1.000 |
| GC-less decode         |  1.803 | 19.726 |           |  32.55 |    0.607 |
| PTE access checks      |  1.763 | 19.128 |           |  33.57 |    0.588 |
| MMU Map fastpath       |  1.709 | 18.507 |           |  34.70 |    0.569 |
| DecodeCFast            |  1.280 | 13.848 | 2m33.240s |  45.78 |    0.426 |
| Cached code page       |  0.977 | 10.582 | 1m57.654s |  60.68 |    0.325 |
| Concrete memory        |  0.915 |  9.835 | 1m48.920s |  65.29 |    0.303 |
| 32-bit decode cache    |  0.684 |  7.327 | 1m20.509s |  87.64 |    0.225 |
| Ld/Sd TLB fastpath     |  0.603 |  6.327 | 1m10.533s | 101.49 |    0.195 |
| Optimized Instr struct |  0.385 |  4.022 | 0m44.167s | 159.65 |    0.124 |
| Interrupts             |  0.418 |  4.357 | 0m49.469s | 147.38 |    0.134 |

# Appendix

## Emulator Example

``` shell
$ cd cmd/goemu/
$ file examples/hello
examples/hello: ELF 64-bit LSB executable, UCB RISC-V, soft-float ABI, version 1 (SYSV), statically linked, not stripped
$ ./goemu -ktrace examples/hello
    0     0 CALL write(1,10050,15)
Hello, RISC-V!
    0     0 RET  write 15
    0     0 CALL exit(0)
```

## Supervisor Mode Example

``` shell
$ cd cmd/goemu/
$ ./goemu  -bios linux-2026-04-08/fw_jump.bin -kernel linux-2026-04-08/Image -initrd linux-2026-04-08/rootfs.cpio.gz -symbols linux-2026-04-08/System.map

OpenSBI v1.6
   ____                    _____ ____ _____
  / __ \                  / ____|  _ \_   _|
 | |  | |_ __   ___ _ __ | (___ | |_) || |
 | |  | | '_ \ / _ \ '_ \ \___ \|  _ < | |
 | |__| | |_) |  __/ | | |____) | |_) || |_
  \____/| .__/ \___|_| |_|_____/|____/_____|
        | |
        |_|


...
[    0.000000] Booting Linux on hartid 0
[    0.000000] Linux version 6.18.7 (root@036cbf3b7083) (riscv64-linux-gcc.br_real (Buildroot 2021.11-18033-g83947c7bb6) 15.1.0, GNU ld (GNU Binutils) 2.44) #1 SMP Wed Apr  8 09:41:06 UTC 2026
[    0.000000] Machine model: goemu,riscv-emulator
[    0.000000] SBI specification v2.0 detected
[    0.000000] SBI implementation ID=0x1 Version=0x10006
[    0.000000] SBI TIME extension detected
[    0.000000] SBI IPI extension detected
[    0.000000] SBI RFENCE extension detected
[    0.000000] SBI DBCN extension detected
[    0.000000] earlycon: sbi0 at I/O port 0x0 (options '')
[    0.000000] printk: legacy bootconsole [sbi0] enabled
...
[   86.116380] NET: Registered PF_INET6 protocol family
[   87.462906] Segment Routing with IPv6
[   87.469746] In-situ OAM (IOAM) with IPv6
[   87.475707] sit: IPv6, IPv4 and MPLS over IPv4 tunneling driver
[   87.513858] NET: Registered PF_PACKET protocol family
[   87.519586] 9pnet: Installing 9P2000 support
[   87.526249] Key type dns_resolver registered
[  139.479820] Freeing initrd memory: 9468K
[  208.722456] clk: Disabling unused clocks
[  208.723727] PM: genpd: Disabling unused power domains
...
[  208.911858] Kernel panic - not syncing: VFS: Unable to mount root fs on unknown-block(0,0)
[  208.913414] ---[ end Kernel panic - not syncing: VFS: Unable to mount root fs on unknown-block(0,0) ]---
```

## Linux image

Converting from `rootfs.ext2` to `rootfs.cpio`:

``` shell
debugfs -R "rdump / ./rootfs_contents" rootfs.ext2
cd rootfs_contents
find . | cpio -o -H newc | gzip > ../rootfs.cpio.gz
```

## Device Tree

``` shell
$ dtc -I dtb -O dts -o source.dts goemu.dtb
```

## Step 2.2 - correctness

### Flw / FmvWX NaN-boxing is wrong

The RISC-V F extension requires that single-precision values stored in
double-precision registers be NaN-boxed: the upper 32 bits must be all
1s. The code does this:

gov64 |= uint64(0xffffffff) << 32

This ORs in the upper bits — but it should be setting the bits
unconditionally, which is fine here since v64 started as
zero-extended. However, the resulting 64-bit pattern is then
interpreted by the host as a double-precision float via
Float64frombits. That bit pattern (0xFFFFFFFF_xxxxxxxx) is a quiet NaN
in IEEE 754 double precision — meaning any subsequent double-precision
operation on this register will silently propagate NaN rather than
operating on the intended single-precision value. The register file
should store the raw bits instead, or FP operations must unwrap the
NaN box. Since FcvtWD just casts cpu.F[instr.Rs1] as float64, loading
a float32 via Flw and then converting with FcvtWD will produce
garbage.

### Div — signed overflow case is missing

The RISC-V spec mandates a special case: INT64_MIN / -1 must return
INT64_MIN (not trap, not undefined). Go's integer division will panic
with a runtime overflow here. The fix:

``` go
case isa.Div:
    rs1 := int64(cpu.X[instr.Rs1])
    rs2 := int64(cpu.X[instr.Rs2])
    if rs2 == 0 {
        cpu.X[instr.Rd] = ^uint64(0)
    } else if rs1 == math.MinInt64 && rs2 == -1 {
        cpu.X[instr.Rd] = uint64(math.MinInt64) // overflow case
    } else {
        cpu.X[instr.Rd] = uint64(rs1 / rs2)
    }
```

The same applies to `Divw` (`INT32_MIN / -1`) and `Rem`/`Remw`.

## Supervisor Model

### Kernel memory map

``` text
+--------------------------------+
| 0xBFFF_FFFF                    |
~                                ~
| 0x8000_0000    System RAM      |
+--------------------------------+
| 0x1000_7FFF                    |
~                                ~
| 0x1000_1000    VirtIO Devices  |
+--------------------------------+
| 0x1000_00FF                    |
~                                ~
| 0x1000_0000    UART (NS16550A) |
+--------------------------------+
| 0x0FFF_FFFF                    |
~                                ~
| 0x0C00_0000    PLIC            |
+--------------------------------+
| 0x0200_FFFF                    |
~                                ~
| 0x0200_0000    CLINT           |
+--------------------------------+
| 0x0000_2FFF                    |
~                                ~
| 0x0000_1000    Boot ROM        |
+--------------------------------+
```

### System RAM

``` text
+--------------------------------+
|                                |
~                                ~
| 0x8800_0000    initrd          |
+--------------------------------+
|                                |
~                                ~
| 0x8400_0000    hw.dtb          |
+--------------------------------+
|                                |
~                                ~
| 0x8020_0000    Image           |
+--------------------------------+
|                                |
~                                ~
| 0x8000_0000    fw_boot.bin     |
+--------------------------------+

```
