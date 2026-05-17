# RISC-V in Go

<p align="center">
  <img src="goemu-small.png" width="320">
</p>

<p align="center">
Building a Linux-capable RV64GC RISC-V machine from scratch in Go,
including CPU emulation, virtual memory, privilege modes, and hardware
devices.
</p>

<p align="center">
  <img src="https://img.shields.io/badge/ISA-RV64GC-blue">
  <img src="https://img.shields.io/badge/Linux-Booting-success">
  <img src="https://img.shields.io/badge/Language-Go-00ADD8">
  <img src="https://img.shields.io/badge/License-MIT-green">
</p>

## Features

- RV64GC instruction set support
- Machine, Supervisor, and User privilege modes
- SV39 virtual memory and page tables
- Linux syscall emulation
- OpenSBI support
- Linux kernel boot support
- Device emulation:
  - NS16550A UART
  - PLIC interrupt controller
  - CLINT / ACLINT timer and IPI devices
- Symbol-aware traces and debugging support
- Instruction-level execution tracing
- Compressed instruction support

## Current status

The emulator currently boots OpenSBI and Linux, reaches initramfs
loading, initializes core devices, and is actively progressing toward
stable userspace execution.

### Userspace

- [x] Standalone binaries
- [x] Statically linked binaries
- [x] Dynamically linked binaries
- [x] Basic Go binaries
- [ ] Full Linux userspace compatibility

### Linux boot

- [x] OpenSBI boot
- [x] Device Tree support
- [x] Supervisor mode
- [x] SV39 MMU
- [x] Linux kernel boot
- [x] Initramfs loading
- [ ] Stable userspace startup
- [ ] Multiprocessor support

## Quick start

Clone and run:

```shell
$ git clone https://github.com/markkurossi/riscv
$ cd riscv/cmd/goemu
$ go build
$ ./goemu -bios linux-2026-04-08/fw_jump.bin \
          -kernel linux-2026-04-08/Image \
          -initrd linux-2026-04-08/rootfs.cpio.gz \
          -symbols linux-2026-04-08/System.map
```

Expected output:

``` shell
OpenSBI v1.6

[    0.000000] Booting Linux on hartid 0
[    0.000000] Linux version 6.18.7 (root@036cbf3b7083) (riscv64-linux-gcc.br_real (Buildroot 2021.11-18033-g83947c7bb6) 15.1.0, GNU ld (GNU Binutils) 2.44) #1 SMP Wed Apr  8 09:41:06 UTC 2026
[    0.000000] Machine model: goemu,riscv-emulator
...
[  139.479820] Freeing initrd memory: 9468K
```

## Architecture

```text
+--------------------+
| Linux / Userspace  |
+--------------------+
| OpenSBI            |
+--------------------+
| Emulator Core      |
|  - RV64GC CPU      |
|  - MMU (SV39)      |
|  - CSR subsystem   |
+--------------------+
| Devices            |
| UART | PLIC        |
| CLINT | VirtIO     |
+--------------------+
```

## Benchmarks

Performance improvements are tracked as the emulator evolves. The
numbers below show the effect of individual optimizations. They are
from running [fibo.c](tests/static/fibo.c) on:

``` text
cpu: Intel(R) Core(TM) i5-8257U CPU @ 1.40GHz
```

| Optimization             |   MIPS |
|:-------------------------|-------:|
| Base                     |  19.75 |
| GC-less decode           |  32.55 |
| PTE access checks        |  33.57 |
| MMU Map fastpath         |  34.70 |
| DecodeCFast              |  45.78 |
| Cached code page         |  60.68 |
| Concrete memory          |  65.29 |
| 32-bit decode cache      |  87.64 |
| Ld/Sd TLB fastpath       | 101.49 |
| Optimized Instr struct   | 159.65 |
| Added interrupt handling | 147.38 |


# Development roadmap

## Near term

 - [ ] Stable Linux userspace startup
 - [ ] Process-local page tables in emulator mode
 - [ ] Full syscall coverage in emulator mode

## Longer term

 - [ ] SMP support
 - [ ] VirtIO devices
 - [ ] Additional ISA extensions

# Appendix

## Benchmark history

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

## Internal roadmap

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
