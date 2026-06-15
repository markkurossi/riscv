# RISC-V in Go

<p align="center">
  <img src="docs/goemu-small.png" width="320">
</p>

<p align="center">
RV64GC RISC-V emulator written in Go capable of booting Linux, NetBSD,
and FreeBSD with OpenSBI, U-Boot, SV39 virtual memory, and VirtIO
devices.
<br>
OpenSBI &#8594; U-Boot &#8594; {BuildRoot,Ubuntu,NetBSD,FreeBSD}
</p>

## Features

- RV64GC instruction set support
- Machine, Supervisor, and User privilege modes
- SV39 virtual memory and page tables
- OpenSBI support
- VirtIO block storage support
- Operating Systems:
  - OpenSBI
  - U-Boot
  - EFI Boot
  - Buildroot Linux
  - Ubuntu 24.04
  - NetBSD 11.99
  - FreeBSD 15.1
- Linux syscall emulation mode
- Device emulation:
  - NS16550A UART
  - PLIC interrupt controller
  - ACLINT MSWI and MTIMER devices
  - Syscon poweroff device
- Symbol-aware traces and debugging support
- Instruction-level execution tracing
- Compressed instruction support

The project is intended as a readable and hackable implementation of a
modern RISC-V platform, suitable for learning emulator internals,
operating systems, and the RISC-V privileged architecture.

## Current status

The emulator successfully boots and runs:

| OS              | Status                                              |
|-----------------|-----------------------------------------------------|
| Buildroot Linux | Shell login                                         |
| Ubuntu 24.04    | Multi-user userspace                                |
| NetBSD 11.99    | Multi-user userspace, package build, clean shutdown |
| FreeBSD 15.1    | Multi-user userspace                                |

Recent milestones:

- OpenSBI 1.6 support
- U-Boot 2026.04 support
- EFI boot support
- SV39 virtual memory
- User/Supervisor/Machine modes
- VirtIO block devices
- NetBSD userspace and C compiler
- FreeBSD userspace
- Clean OS shutdown via SBI

![Linux Boot](docs/linux-shell.png)

### Userspace emulation

- [x] Standalone binaries
- [x] Statically linked binaries
- [x] Dynamically linked binaries
- [x] Basic Go binaries
- [ ] Full Linux userspace compatibility

### System emulation and supervisor mode

- [ ] ACLINT timer/IPI support
- [ ] Parse kernel PE32+ header: load address, symbols
- [ ] VirtIO
  - [x] virtio-blk
  - [x] virtio-rng
  - [ ] virtio-console
  - [ ] virtio-net
- [ ] Rewrite UART with FIFOs
- [ ] [riscv-arch-test](https://github.com/riscv/riscv-arch-test)
- [x] Initramfs loading
- [x] Buildroot shell login
- [x] System shutdown support
- [ ] SMP support

## Quick start

Clone and run:

```shell
$ git clone https://github.com/markkurossi/riscv
$ cd riscv/cmd/goemu
$ go build
$ ./goemu buildroot.goemu
```

Expected output:

``` shell
OpenSBI v1.6

[    0.000000] Booting Linux on hartid 0
[    0.000000] Linux version 7.0.11 (root@5c73f8dabee1) (riscv64-linux-gnu-gcc (Ubuntu 13.3.0-6ubuntu2~24.04.1) 13.3.0, GNU ld (GNU Binutils for Ubuntu) 2.42) #9 SMP PREEMPT Wed Jun  3 08:23:43 UTC 2026
[    0.000000] Machine model: goemu,riscv-emulator
...
[   12.291753] Freeing initrd memory: 9468K
[   18.570200] clk: Disabling unused clocks
[   18.570327] PM: genpd: Disabling unused power domains
[   18.570487] ALSA device list:
[   18.570595]   No soundcards found.
[   18.582016] Freeing unused kernel image (initmem) memory: 2428K
[   18.582810] Run /init as init process
...
Welcome to GoEMU RISC-V Linux

Kernel: 7.0.11
Machine: riscv64

buildroot login: root
login[79]: root login on 'console'
# ls -la
total 4
drwx------    2 root     root            60 Apr 19 05:25 .
drwxr-xr-x   18 root     root           420 May 18 05:41 ..
-rw-------    1 root     root           192 Apr 19 05:25 .ash_history
# uname -a
Linux goemu 7.0.11 #9 SMP PREEMPT Wed Jun  3 08:23:43 UTC 2026 riscv64 GNU/Linux
# date
Mon Jun  8 04:55:00 UTC 2026
# halt
...
The system is going down NOW!
Sent SIGTERM to all processes
Sent SIGKILL to all processes
Requesting system halt
[   22.943205] reboot: System halted
```

The `buildroot.goemu` file defines the emulator BIOS, kernel, and
rootfs arguments. Those arguments can also be specified (and
overwritten) with corresponding command line arguments:

``` shell
$ ./goemu -bios opensbi/fw_jump.bin \
          -kernel linux-7.0.11/Image \
          -symbols linux-7.0.11/System.map \
          -drive id=hd0,format=raw,file=linux-2026-04-08/rootfs.ext2 \
          -device virtio-blk-device,drive=hd0 \
          -append "earlycon=sbi console=ttyS0,115200 root=/dev/vda ro rootwait"
```

## Emulator Example

The `goemu` provides also a primitive Linux userspace emulation:

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

## Architecture

```text
+--------------------+
| Linux / NetBSD /   |
| FreeBSD Userspace  |
+--------------------+
| Linux / NetBSD /   |
| FreeBSD Kernel     |
+--------------------+
| U-Boot / EFI       |
+--------------------+
| OpenSBI            |
+--------------------+
| RV64GC CPU         |
| SV39 MMU           |
| CSR subsystem      |
| Interrupts         |
+--------------------+
| VirtIO Block       |
| UART               |
| PLIC               |
| ACLINT             |
| Goldfish RTC       |
| Syscon Poweroff    |
+--------------------+
```

## Benchmarks

## Real Workloads

NetBSD 11.99:

- Boot to login prompt
- Compile Hello World with GCC
- Run user binaries
- Multi-minute uptime
- Clean shutdown

Example:

``` shell
$ time cc -o hello hello.c
33.57 real
32.15 user
1.30 sys
```

## Fibonacci

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

 - [ ] VirtIO networking
 - [ ] VirtIO RNG
 - [ ] Process-local page tables in emulator mode
 - [ ] Full syscall coverage in emulator mode

## Longer term

 - [ ] SMP support
 - [ ] Additional VirtIO devices
 - [ ] Additional ISA extensions
 - [ ] JIT experiments
 - [ ] Performance optimizations

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
| Interrupts             |  0.416 |  4.321 | 0m49.085s | 148.61 |    0.134 |

## Internal roadmap

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

## Step 2 - MMU and Linux syscalls

 - [x] MMU with page tables
   - [ ] Move pagetable to processes
 - [ ] Refactor emulator source files
 - [ ] Rethink FD handling.
 - [ ] Run most Linux and Go binaries
 - [ ] Proper Linux syscall support

## Step 3 - Supervisor mode

 - [x] Supervisor mode
 - [x] Boot Linux kernel

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
# OpenSBI and Das U-Boot

## u-boot

``` shell
$ apt-get update && apt-get install -y libssl-dev
$ apt-get update && apt-get install -y libgnutls28-dev uuid-dev
```

``` shell
$ cd u-boot-2026.04/
$ export CROSS_COMPILE=riscv64-linux-gnu-
$ ls configs/ | grep riscv
openpiton_riscv64_defconfig
openpiton_riscv64_spl_defconfig
qemu-riscv64_defconfig
qemu-riscv64_smode_acpi_defconfig
qemu-riscv64_smode_defconfig
qemu-riscv64_spl_defconfig
$ make qemu-riscv64_smode_defconfig
  HOSTCC  scripts/basic/fixdep
  HOSTCC  scripts/kconfig/conf.o
  YACC    scripts/kconfig/zconf.tab.[ch]
  LEX     scripts/kconfig/zconf.lex.c
  HOSTCC  scripts/kconfig/zconf.tab.o
  HOSTLD  scripts/kconfig/conf
#
# configuration written to .config
#
$ make -j$(nproc)
```

# NetBSD

``` shell
$ ./goemu netbsd.goemu
<RET>
fatload virtio 0:1 0x84000000 /EFI/BOOT/BOOTRISCV64.EFI
setenv bootargs "boot hd0a:netbsd -v -s consdev=com0 speed=115200"
bootefi 0x84000000 0x9eeae220
```

debugging in u-boot:

``` shell
=> part list virtio 0
```

# FreeBSD

Press space to enter FreeBSD boot console:

``` shell
<SPACE>
OK set boot_verbose=1
OK boot
```

## Boot log

``` shell
$ ./goemu -log /tmp/trace.sock freebsd.goemu
Remote logger enabled

OpenSBI v1.6
   ____                    _____ ____ _____
  / __ \                  / ____|  _ \_   _|
 | |  | |_ __   ___ _ __ | (___ | |_) || |
 | |  | | '_ \ / _ \ '_ \ \___ \|  _ < | |
 | |__| | |_) |  __/ | | |____) | |_) || |_
  \____/| .__/ \___|_| |_|_____/|____/_____|
        | |
        |_|

Platform Name               : goemu,riscv-emulator
Platform Features           : medeleg
Platform HART Count         : 1
Platform IPI Device         : aclint-mswi
Platform Timer Device       : aclint-mtimer @ 100000000Hz
Platform Console Device     : uart8250
Platform HSM Device         : ---
Platform PMU Device         : ---
Platform Reboot Device      : ---
Platform Shutdown Device    : syscon-poweroff
Platform Suspend Device     : ---
Platform CPPC Device        : ---
Firmware Base               : 0x80000000
Firmware Size               : 325 KB
Firmware RW Offset          : 0x40000
Firmware RW Size            : 69 KB
Firmware Heap Offset        : 0x48000
Firmware Heap Size          : 37 KB (total), 2 KB (reserved), 13 KB (used), 21 KB (free)
Firmware Scratch Size       : 4096 B (total), 440 B (used), 3656 B (free)
Runtime SBI Version         : 2.0
Standard SBI Extensions     : time,rfnc,ipi,base,hsm,srst,pmu,dbcn,legacy
Experimental SBI Extensions : fwft,dbtr,sse

Domain0 Name                : root
Domain0 Boot HART           : 0
Domain0 HARTs               : 0*
Domain0 Region00            : 0x0000000010000000-0x0000000010000fff M: (I,R,W) S/U: (R,W)
Domain0 Region01            : 0x0000000002000000-0x000000000200ffff M: (I,R,W) S/U: ()
Domain0 Region02            : 0x0000000080040000-0x000000008005ffff M: (R,W) S/U: ()
Domain0 Region03            : 0x0000000080000000-0x000000008003ffff M: (R,X) S/U: ()
Domain0 Region04            : 0x000000000c000000-0x000000000c3fffff M: (I,R,W) S/U: (R,W)
Domain0 Region05            : 0x0000000000000000-0xffffffffffffffff M: () S/U: (R,W,X)
Domain0 Next Address        : 0x0000000080200000
Domain0 Next Arg1           : 0x0000000082200000
Domain0 Next Mode           : S-mode
Domain0 SysReset            : yes
Domain0 SysSuspend          : yes

Boot HART ID                : 0
Boot HART Domain            : root
Boot HART Priv Version      : v1.12
Boot HART Base ISA          : rv64imafdcg
Boot HART ISA Extensions    : smaia,smstateen,sscofpmf,sstc,zicntr,zihpm,smcntrpmf,sdtrig
Boot HART PMP Count         : 64
Boot HART PMP Granularity   : 2 bits
Boot HART PMP Address Bits  : 54
Boot HART MHPM Info         : 0 (0x00000000)
Boot HART Debug Triggers    : 1 triggers
Boot HART MIDELEG           : 0x0000000000002222
Boot HART MEDELEG           : 0x000000000004b109


U-Boot 2026.04 (Jun 08 2026 - 20:44:13 +0000)

CPU:   riscv
Model: goemu,riscv-emulator
DRAM:  512 MiB
using memory 0x9eeaf000-0x9f6cf000 for malloc()
Core:  18 devices, 12 uclasses, devicetree: board
Loading Environment from nowhere... OK
In:    serial,usbkbd
Out:   serial,vidconsole
Err:   serial,vidconsole
No USB controllers found
Net:   No ethernet found.

Working FDT set to 9eeae1c0
Hit any key to stop autoboot: 0

Device 0: unknown device

Device 0: UMoG VirtIO Block Device
            Type: Hard Disk
            Capacity: 6144.0 MB = 6.0 GB (12582912 x 512)
... is now current device
Scanning virtio 0:3...
Failed to load '/'
Failed to load '/dtb/'
Booting: Label: virtio 0 Device path: /VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,0000000000000000)/VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,7200000000000000)/VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,8b00000001000000)/VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,17008b0000000000)

























Consoles: EFI console
    Reading loader env vars from /efi/freebsd/loader.env
Setting currdev to disk0p3:
FreeBSD/riscv EFI loader, Revision 3.0

   Command line arguments: l
   Image base: 0x9dd1a000
   EFI version: 2.110
   EFI Firmware: Das U-Boot (rev 8230.1024)
   Console: comconsole (0)
   Load Path: /\EFI\BOOT\BOOTRISCV64.EFI
   Load Device: /VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,0000000000000000)/VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,7200000000000000)/VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,8b00000001000000)/VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,17008b0000000000)/HD(3,GPT,f2b5282b-616a-11f1-a254-0cc47ad8b808,0x4000,0x1b000)
   BootCurrent: 0000
   BootOrder: 0000[*]
   BootInfo Path: /VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,0000000000000000)/VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,7200000000000000)/VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,8b00000001000000)/VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,17008b0000000000)
Ignoring Boot0000: Only one DP found
Trying ESP: /VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,0000000000000000)/VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,7200000000000000)/VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,8b00000001000000)/VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,17008b0000000000)/HD(3,GPT,f2b5282b-616a-11f1-a254-0cc47ad8b808,0x4000,0x1b000)
Setting currdev to disk0p3:
Trying: /VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,0000000000000000)/VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,7200000000000000)/VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,8b00000001000000)/VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,17008b0000000000)/HD(1,GPT,f2b33ab9-616a-11f1-a254-0cc47ad8b808,0x1000,0x1000)
Setting currdev to disk0p1:
Trying: /VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,0000000000000000)/VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,7200000000000000)/VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,8b00000001000000)/VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,17008b0000000000)/HD(2,GPT,f2b4338c-616a-11f1-a254-0cc47ad8b808,0x2000,0x2000)
Setting currdev to disk0p2:
Trying: /VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,0000000000000000)/VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,7200000000000000)/VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,8b00000001000000)/VenHw(e61d73b9-a384-4acc-aeab-82e828f3628b,17008b0000000000)/HD(4,GPT,f2b60a1b-616a-11f1-a254-0cc47ad8b808,0x1f000,0xbe0f80)
Setting currdev to disk0p4:
Loading /boot/defaults/loader.conf
Loading /boot/defaults/loader.conf
Loading /boot/device.hints
Loading /boot/loader.conf
Loading /boot/loader.conf.local
-
Loading kernel...
/boot/kernel/kernel text=0x62746c text=0x168590 text=0x60 data=0x1187f8 data=0xf00+0x1e6cc0 0x8+0x127dd0+0x8+0x10cc31
Loading configured modules...
/boot/kernel/umodem.ko text=0x20c0 text=0x1290 data=0x700+0x4 0x8+0x6900+0x8+0xf04
loading required module 'ucom'
/boot/kernel/ucom.ko text=0x254c text=0x3074 data=0x948+0x858 0x8+0x120a8+0x8+0x16e8
/etc/hostid size=0x25
/boot/entropy size=0x1000

Hit [Enter] to boot immediately, or any other key for command prompt.
Booting [/boot/kernel/kernel]...
Using DTB provided by EFI at 0x9de74000.
Kernel args: (null)
Loading splash ok
Loading shutdown splash ok
---<<BOOT>>---
Copyright (c) 1992-2025 The FreeBSD Project.
Copyright (c) 1979, 1980, 1983, 1986, 1988, 1989, 1991, 1992, 1993, 1994
	The Regents of the University of California. All rights reserved.
FreeBSD is a registered trademark of The FreeBSD Foundation.
FreeBSD 15.1-RC3 releng/15.1-n283548-9263fb9bab26 GENERIC riscv
FreeBSD clang version 19.1.7 (https://github.com/llvm/llvm-project.git llvmorg-19.1.7-0-gcd708029e0b2)
VT: init without driver.
SBI: OpenSBI v1.6
SBI Specification Version: 2.0
CPU 0  : Vendor=Unspecified Core=Unknown (Hart 0)
  marchid=0x100, mimpid=0x1
  MMU: 0x1<Sv39>
  ISA: 0x112d<Atomic,Compressed,Double,Float,Mult/Div>
  S-mode Extensions: 0
real memory  = 536477696 (511 MB)
avail memory = 503767040 (480 MB)
random: unblocking device.
random: entropy device external interface
kbd0 at kbdmux0
ofwbus0: <Open Firmware Device Tree>
simplebus0: <Flattened device tree simple bus> on ofwbus0
simple_mfd0: <Simple MFD (Multi-Functions Device)> mem 0x10000100-0x100001ff on simplebus0
sbi0: <RISC-V Supervisor Binary Interface>
cpulist0: <Open Firmware CPU Group> on ofwbus0
cpu0: <Open Firmware CPU> on cpulist0
intc0: <RISC-V Local Interrupt Controller> on ofwbus0
sbi_ipi0: <RISC-V SBI Inter-Processor Interrupts> on sbi0
plic0: <RISC-V PLIC> mem 0xc000000-0xc3fffff irq 2,3 on simplebus0
timer0: <RISC-V Timer>
Timecounter "RISC-V Timecounter" frequency 100000000 Hz quality 1000
Event timer "RISC-V Eventtimer" frequency 100000000 Hz quality 1000
rcons0: <RISC-V console>
uart0: <Non-standard ns8250 class UART with FIFOs> mem 0x10000000-0x100000ff irq 4 on simplebus0
uart0: console (418,n,8,1)
syscon_power0: <Syscon poweroff> on simplebus0
goldfish_rtc0: <Goldfish RTC> mem 0x10100000-0x10100fff irq 5 on simplebus0
goldfish_rtc0: registered as a time-of-day clock, resolution 1.000000s
virtio_mmio0: <VirtIO MMIO adapter> mem 0x10008000-0x100080ff irq 6 on simplebus0
virtio_mmio1: <VirtIO MMIO adapter> mem 0x10008100-0x100090ff irq 7 on simplebus0
vtblk0: <VirtIO Block Adapter> on virtio_mmio1
vtblk0: 6144MB (12582912 512 byte sectors)
Timecounters tick every 1.000 msec
usb_needs_explore_all: no devclass
Trying to mount root from ufs:/dev/ufs/rootfs [rw]...
WARNING: / was not properly dismounted
Setting hostuuid: e2f3526f-a90b-
```
