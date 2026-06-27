# RISC-V in Go

<p align="center">
  <img src="docs/goemu-small.png" width="320">
</p>

<p align="center">
RV64GC RISC-V emulator written in Go capable of booting Linux, NetBSD,
and FreeBSD userspace environments with OpenSBI, U-Boot, SV39 virtual
memory, and VirtIO devices
</p>

<p align="center">
OpenSBI &#8594; BuildRoot Linux<br>
OpenSBI &#8594; U-Boot &#8594; EFI &#8594; {NetBSD,FreeBSD}<br>
OpenSBI &#8594; U-Boot &#8594; EFI &#8594; GRUB &#8594; Ubuntu Linux
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

![Linux Boot](docs/linux-shell.png)
![NetBSD](docs/netbsd.png)
![FreeBSD](docs/freebsd.png)

### System emulation and supervisor mode

- [ ] VirtIO
  - [x] virtio-blk
  - [x] virtio-rng
  - [ ] virtio-net
    - [x] Network with tun devices.
    - [ ] DHCP server
    - [ ] HTTP proxy
    - [ ] NTP server
  - [ ] virtio-gpu
  - [ ] virtio-console
- [ ] [riscv-arch-test](https://github.com/riscv/riscv-arch-test)
- [ ] SMP support
- [ ] ACLINT timer/IPI support
- [ ] Parse Linux kernel PE32+ header: load address, symbols
- [ ] JIT experiments

### Userspace emulation

- [ ] Full Linux userspace compatibility

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

The `cmd/goemu` directory contains `*.goemu` files for different
operation systems. The use OpenSBI and U-Boot for the boot process but
you must load the corresponding Operating System image to boot.

 - [BuildRoot](cmd/goemu/buildroot.goemu) - self-contained
 - [FreeBSD](cmd/goemu/freebsd.goemu) - download FreeBSD-15.1-RELEASE
   image
 - [NetBSD](cmd/goemu/netbsd.goemu) - download NetBSD 11.99 image
 - [Ubuntu-24.04](cmd/goemu/u-boot-ubuntu-24-04.goemu) - download
   Ubuntu-24.04.4 image

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

## Userspace Emulation - MMU Refactoring

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
 - [x] MMU with page tables
   - [ ] Move pagetable to processes
 - [ ] Refactor emulator source files
 - [ ] Rethink FD handling.
 - [ ] Run most Linux and Go binaries
 - [ ] Proper Linux syscall support

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

## Network routing

On macOS host:

``` shell
$ sudo sysctl -w net.inet.ip.forwarding=1
```

Create `pf.conf`

``` txt
nat on en0 from 192.168.42.0/24 to any -> (en0)
```

Load conf:

``` shell
$ sudo pfctl -f pf.conf
$ sudo pfctl -e
```

## pkg

``` shell
pkg_env: {
    http_proxy: "http://myproxy:3128",
}
```

## ports

``` shell
$ tar -xf ports-main.tar.gz -C /usr
$ mv /usr/ports-main /usr/ports
$ cd /usr/ports/x11/xorg
$ make install clean
```

### graphics/feh

If you need an actual interactive viewer (to zoom, scale, or close out
of the image with a hotkey), you can use graphics/feh. While feh
typically expects an X server, it can run directly over the vt(4)
console framebuffer if paired with SDL or a minimalist backend.

``` shell
$ feh --geometry 1920x1080 image.png
```

(Replace `1920x1080` with your actual console's pixel resolution).

## Compiling git

### Compile curl

``` shell
$ ./configure --with-openssl --without-libpsl
```

### Compile git

The `-O0` is needed to prevent clang compiler error:

``` shell
CC sha1dc/sha1.o

error: ran out of registers during register allocation

1 error generated.
```

``` shell
$ ./configure --with-curl=/usr/local
$ gmake NO_RUST=1 CFLAGS="-O0 -I/usr/local/include" LDFLAGS="-L/usr/local/lib -liconv"
```

## Expanding image

Host:

``` shell
$ truncate -s +1G FreeBSD-15.1-RELEASE-riscv-riscv64-GENERICSD.img
```

Guest:

``` shell
# gpart recover vtbd0
# gpart resize -i 4 vtbd0
# growfs /
```

# Ubuntu

## Grow Image

Host:

``` shell
# Add 5 GB to the image
$ truncate -s +5G rootfs.img
```

Guest:

``` shell
$ df -h
$ lsblk -f
$ sudo growpart /dev/vda 1
$ sudo resize2fs /dev/vda1
```
