# RISC-V in Go

<p align="center">
  <img src="goemu-small.png" width="320">
</p>

<p align="center">
Linux-capable RV64GC RISC-V emulator written in Go with SV39 virtual
memory, privilege modes, and device emulation.
</p>

## Features

- RV64GC instruction set support
- Machine, Supervisor, and User privilege modes
- SV39 virtual memory and page tables
- OpenSBI support
- Linux kernel boot support
- Linux syscall emulation mode
- Device emulation:
  - NS16550A UART
  - PLIC interrupt controller
  - ACLINT MSWI and MTIMER devices
  - Syscon poweroff device
- Symbol-aware traces and debugging support
- Instruction-level execution tracing
- Compressed instruction support

## Current status

The emulator now boots OpenSBI and Linux 6.x to a functional Buildroot
shell. The machine supports privilege transitions, virtual memory,
interrupts, timer devices, and enough platform hardware to run a Linux
userspace environment.

Current development has shifted from "make Linux boot" to platform
completeness, additional devices, and performance work.

### Userspace emulation

- [x] Standalone binaries
- [x] Statically linked binaries
- [x] Dynamically linked binaries
- [x] Basic Go binaries
- [ ] Full Linux userspace compatibility

### Linux system emulation

- [x] OpenSBI boot
- [x] Device Tree support
- [x] Machine/Supervisor/User privilege modes
- [x] SV39 MMU
- [x] Interrupt handling
- [ ] ACLINT timer/IPI support
- [x] PLIC interrupt controller
- [ ] Parse kernel PE32+ header: load address, symbols
- [ ] VirtIO
- [x] Initramfs loading
- [x] Buildroot shell login
- [x] System shutdown support
- [ ] SMP support

## [OASIS VirtIO (Virtual I/O)](https://www.oasis-open.org/standard/virtio-v1-1/)

| Device                        | Linux Driver   | Device ID |
| :-----                        | :----          | -----:    |
| Block Device                  | virtio_blk     | 2         |
| 9P Transport                  | 9pnet_virtio   | 9         |
| Network Card                  | virtio_net     | 1         |
| Entropy Source / RNG          | virtio_rng     | 4         |
| Cryptographic Accelerator     | virtio_crypto  | 20        |
| Persistent Error Storage      | virtio_pstore  | 22        |
| Graphics Adapter / GPU        | virtio_gpu     | 16        |
| Input Subsystem               | virtio_input   | 18        |
| Console / Multi-Stream Serial | virtio_console | 3         |

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
[   12.291753] Freeing initrd memory: 9468K
[   18.570200] clk: Disabling unused clocks
[   18.570327] PM: genpd: Disabling unused power domains
[   18.570487] ALSA device list:
[   18.570595]   No soundcards found.
[   18.582016] Freeing unused kernel image (initmem) memory: 2428K
[   18.582810] Run /init as init process
...
Welcome to Buildroot
buildroot login: root
login[79]: root login on 'console'
# ls -la
total 4
drwx------    2 root     root            60 Apr 19 05:25 .
drwxr-xr-x   18 root     root           420 May 18 05:41 ..
-rw-------    1 root     root           192 Apr 19 05:25 .ash_history
# uname -a
Linux buildroot 6.18.7 #1 SMP Wed Apr  8 09:41:06 UTC 2026 riscv64 GNU/Linux
# date
Thu May 21 20:06:59 UTC 2026
# halt
...
The system is going down NOW!
Sent SIGTERM to all processes
Sent SIGKILL to all processes
Requesting system halt
[   22.943205] reboot: System halted
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
|  - Trap handling   |
+--------------------+
| Devices            |
| UART               |
| PLIC               |
| ACLINT             |
| Syscon             |
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

 - [x] UART interrupt wiring in DTB
 - [x] Host terminal polling
 - [ ] Proper wfi sleep behavior
 - [ ] Add VirtIO block device (virtio-blk)
 - [ ] Add VirtIO networking
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

### Mode Transition

In RISC-V, mode transitions are governed by privileged instructions
(`ecall`, `ebreak`, `sret`, `mret`) and hardware interrupts. When an
exception or interrupt occurs, the processor transitions to a higher
(or equal) privilege mode to handle it. Conversely, returning from a
handler restores the previous state.

Here is the foundational breakdown of how these transitions work,
filling out the table for **User (U)**, **Supervisor (S)**, and
**Machine (M)** modes.

#### Key Concepts & CSR Fields

Before looking at the table, it helps to understand what the actions
mean:

* **`xPP` (Previous Privilege):** Holds the mode the processor was in
  *before* the trap occurred ($U=00$, $S=01$, $M=11$).
* **`xPIE` (Previous Interrupt Enable):** Saves the state of the
  interrupt enable bit (`xIE`) from before the trap.
* **`xIE` (Interrupt Enable):** Gets set to `0` during a trap to
  globally disable interrupts while entering the handler.
* **`xtvec` / `xepc`:** The target address is determined by the trap
  vector register (`mtvec`/`stvec`), and the return address is saved
  in the exception program counter (`mepc`/`sepc`).

### RISC-V State Transition Table

By default, traps go to **M-mode** unless they are explicitly
delegated to **S-mode** using delegation registers (`medeleg` for
exceptions, `mideleg` for interrupts).

| Input  | Mode | Cond   | Cause  | Actions                             | Tgt |
|--------|------|--------|--------|-------------------------------------|-----|
| ecall  | M    | —      | EcallM | MPP=M, MPIE=MIE, MIE=0, mepc=pc     | M   |
| ecall  | S    | !Deleg | EcallS | MPP=S, MPIE=MIE, MIE=0, mepc=pc     | M   |
| ecall  | S    | Deleg  | EcallS | SPP=S, SPIE=SIE, SIE=0, sepc=pc     | S   |
| ecall  | U    | !Deleg | EcallU | MPP=U, MPIE=MIE, MIE=0, mepc=pc     | M   |
| ecall  | U    | Deleg  | EcallU | SPP=U, SPIE=SIE, SIE=0, sepc=pc     | S   |
| ebreak | Any  | !Deleg | Break  | MPP=mode, MPIE=MIE, MIE=0, mepc=pc  | M   |
| ebreak | Any  | Deleg  | Break  | SPP=mode, SPIE=SIE, SIE=0, sepc=pc  | S   |
| mret   | M    | —      | —      | MIE=MPIE, MPIE=1, mode=MPP, pc=mepc | MPP |
| sret   | S    | —      | —      | SIE=SPIE, SPIE=1, mode=SPP, pc=sepc | SPP |
| Intr   | Any  | !Deleg | Intr   | MPP=mode, MPIE=MIE, MIE=0, mepc=pc  | M   |
| Intr   | Any  | Deleg  | Intr   | SPP=mode, SPIE=SIE, SIE=0, sepc=pc  | S   |


### Key Takeaways from the Table

1. **The Delegation Rule:** Notice how for `U` and `S` modes, the
   outcome depends on whether the trap is delegated. If
   `medeleg[cause]` or `mideleg[cause]` is set to 1, the trap bypasses
   M-mode entirely and updates the **S-mode** CSRs (`sstatus`, `sepc`,
   `stvec`).

2. **The Return Mechanism (`xret`):** When executing `mret`, the
   hardware reads the `MPP` field to know what mode to drop back
   into. It also restores the interrupt state (`MIE = MPIE`) and
   resets `MPIE` to 1.

3. **Interrupts vs. Exceptions:** An `ecall` or `ebreak` is
   synchronous (the saved `epc` points to the instruction itself). An
   interrupt is asynchronous, meaning `epc` points to the next
   instruction that *would* have been executed.

4. **M-Mode Interrupts:** If an interrupt occurs while already running
   in M-mode, it cannot be delegated to S-mode (global RISC-V rule:
   traps can never be delegated to a lower privilege mode than the one
   they occurred in). The table still holds true because Deleg
   implicitly evaluates to false if Mode == M.

### M and S mode trap CSR blocks

The full M-mode trap CSR block is:

| CSR        | Address | Purpose                                              |
|------------|---------|------------------------------------------------------|
| `mstatus`  | `0x300` | Global status (MPP, MIE, MPIE, etc.)                 |
| `mtvec`    | `0x305` | Trap vector base address                             |
| `mscratch` | `0x340` | Scratch register for M-mode handler                  |
| `mepc`     | `0x341` | PC of trapping instruction                           |
| `mcause`   | `0x342` | Exception/interrupt cause code                       |
| `mtval`    | `0x343` | Trap value (faulting address, bad instruction, etc.) |
| `mip`      | `0x344` | Machine interrupt pending                            |

And the S-mode equivalents (for completeness):

| CSR        | Address | Purpose                             |
|------------|---------|-------------------------------------|
| `sstatus`  | `0x100` | Subset of mstatus visible to S-mode |
| `stvec`    | `0x105` | S-mode trap vector                  |
| `sscratch` | `0x140` | Scratch for S-mode handler          |
| `sepc`     | `0x141` | S-mode exception PC                 |
| `scause`   | `0x142` | S-mode cause                        |
| `stval`    | `0x143` | S-mode trap value                   |
| `sip`      | `0x144` | S-mode interrupt pending            |

### MMU

| Mode | PTE (U, R, W, X)   | SUM      | MXR      | Read    | Store   | Exec |
| ---- | :----------------- | :------- | :------- | :------ | :------ | :--- |
| U    | U=0 (Any R/W/X)    | —        | —        | No      | No      | No   |
| U    | U=1, R=1, W=1, X=0 | —        | —        | yes     | yes     | No   |
| U    | U=1, R=0, W=0, X=1 | —        | MXR=0    | No      | No      | yes  |
| U    | U=1, R=0, W=0, X=1 | —        | MXR=1    | yes     | No      | yes  |
| S    | U=1 (Any R/W/X)    | SUM=0    | —        | No      | No      | No   |
| S    | U=1, R=1, W=1, X=0 | SUM=1    | —        | yes     | yes     | No   |
| S    | U=1, R=0, W=0, X=1 | SUM=1    | MXR=0    | No      | No      | No¹  |
| S    | U=0, R=1, W=0, X=1 | —        | MXR=0    | yes     | No      | yes  |
| S    | U=0, R=0, W=0, X=1 | —        | MXR=1    | yes     | No      | yes  |
| M    | (Any Page)         | —        | —        | yes²    | yes²    | yes² |
| M    | (Any Page)         | mstatus³ | mstatus³ | Match S | Match S | No   |

**Architectural Rules & Pitfalls**

1. The S-Mode Execution Trap: Notice row 7. Even if SUM=1, an attempt
   by Supervisor mode to fetch an instruction from a User page (U=1)
   will always trigger an instruction page fault. This is a hardcoded
   security feature to prevent "ret2usr" exploits where a kernel is
   tricked into running malicious user-space code.

2. M-Mode and the MPRV Exception: Normally, if the Modify Privilege
   bit is unset (mstatus.MPRV=0), Machine mode bypasses the MMU
   entirely.

2. However, if the Modify Privilege bit is set (mstatus.MPRV=1), the
   MMU steps in only for data loads and stores (not fetches). It
   translates those accesses using the privilege level specified in
   mstatus.MPP (usually S or U mode), meaning SUM and MXR suddenly
   apply to M-mode data operations too. The `Read` and `Store`
   operations match S-mode semantics.


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

### The Boot Sequence

1. **Prepare the DTB (Device Tree Blob):** This is a small data
   structure that tells Linux "The UART is at 0x10000000 and I have
   512MB of RAM."
2. **Load the Image:** Put the Linux `Image` binary at `0x80200000`.
3. **Set Initial Registers:**
   * `a0 = 0` (The Hart ID)
   * `a1 = 0x82000000` (Address of the DTB)
   * `PC = 0x80200000`


# Device Tree Blob (DTB)

The canonical documentation for the Device Tree Blob (DTB) format is
the **Devicetree Specification**, currently maintained by
[Devicetree.org](https://www.devicetree.org). Specifically, **Chapter
5: Flattened Devicetree (DTB) Format** contains the exact memory
layout.

The DTB (also known as a Flattened Devicetree or FDT) is a linear,
pointerless data structure. When loaded into memory, it must follow a
specific sequence of blocks.

### 1. High-Level Memory Layout

A DTB file consists of four main sections, which must appear in the
following order:

1. **fdt_header**: A fixed-size header containing magic numbers and offsets.
2. **memory reservation block**: A list of memory areas the kernel must not use.
3. **structure block**: The actual tree (nodes and properties) encoded as a series of tokens.
4. **strings block**: All property names are stored here as null-terminated strings to save space.

---

### 2. The Header (`fdt_header`)

The header is the "entry point" for any parser. **Note:** All fields
are 32-bit integers stored in **Big-Endian** format.

| Field           | Offset | Description                                    |
| ---             | ---    | ---                                            |
| magic           | 0x00   | The constant 0xd00dfeed                        |
| totalsize       | 0x04   | Total size of the DTB in bytes                 |
| off_dt_struct   | 0x08   | Offset from header to Structure Block          |
| off_dt_strings  | 0x0C   | Offset from header to Strings Block.           |
| off_mem_rsvmap  | 0x10   | Offset from header to Memory Reservation Block |
| version         | 0x14   | Format version (standard is `17`)              |
| last_comp_ver   | 0x18   | Last compatible version (usually `16`)         |
| boot_cpuid_phys | 0x1C   | Physical ID of the system's boot CPU           |
| size_dt_strings | 0x20   | Length of the Strings Block in bytes           |
| size_dt_struct  | 0x24   | Length of the Structure Block in bytes         |

---

### 3. Structure Block (The "Tree")

The tree is parsed as a stream of tokens. Each token is a 32-bit
Big-Endian integer:

* **`FDT_BEGIN_NODE` (0x00000001)**: Followed by the null-terminated name of the node (padded to 4-byte alignment).
* **`FDT_END_NODE` (0x00000002)**: No data follows.
* **`FDT_PROP` (0x00000003)**: Followed by:
1. `uint32 len`: Length of the property's value.
2. `uint32 nameoff`: Offset within the **Strings Block** for the property name.
3. `data`: The actual value bytes (padded to 4-byte alignment).


* **`FDT_NOP` (0x00000004)**: Ignored.
* **`FDT_END` (0x00000009)**: Ends the entire structure block.

### 4. Implementation Notes

The emulator is targeting RISC-V, note:

1. **Endianness**: emulator's internal memory is Little-Endian (RISC-V
   standard), but the DTB is **strictly Big-Endian**. Use
   `binary.BigEndian` to read the header and tokens.
2. **Alignment**: Every section and property value must be aligned to
   a **4-byte boundary**. If a node name is "uart", it takes 4 bytes +
   1 null terminator = 5 bytes; you must skip 3 padding bytes before
   the next token.
3. **Passing to Linux**: The RISC-V ABI requires qthe **physical
   address** of the DTB header to be stored into register **`a1`**.

> **Canonical URL**: You can find the latest stable version (v0.4) of the full specification here: [https://www.devicetree.org/specifications/](https://www.devicetree.org/specifications/)
