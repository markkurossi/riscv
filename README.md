# RISC-V in Go

<img align="center" src="goemu-small.png">

A RISC-V emulator written in Go. The goal is to implement the RV64GC
(64-bit, general-purpose, compressed) profile, with support for running
Linux applications and, eventually, the Linux operating system.

## MMU Refactoring

 - [x] Fix page table MMU code to run the existing samples
 - [ ] Refactor the whole emulator chain:
   - [ ] CPU -> Kernel -> Process -> Emulator
   - [ ] Kernel creates processes 1-n
   - [ ] Virtual memory handled per process
   - [ ] CPU has only {Load,Store}Uint{8,16,32,64}()
   - [ ] Process has {Put,}Uint{8,16,32,64,String,Data}() functions
 - [ ] sfence.vma must clearn MMU's TLB entries

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

### 5. Div — signed overflow case is missing

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

## Step 3 - Supervisor mode

 - [ ] Supervisor mode
 - [ ] Boot Linux kernel

# Benchmarks

| Optimization | fib 30 | fib 35 | fib 40 |
|:-------------|-------:|-------:|-------:|
| Base         |  2.969 | 32.510 |        |

# Appendix

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
