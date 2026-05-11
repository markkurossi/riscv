# RISC-V in Go

<img align="center" src="goemu-small.png">

A RISC-V emulator written in Go. The goal is to implement the RV64GC
(64-bit, general-purpose, compressed) profile, with support for running
Linux applications and, eventually, the Linux operating system.

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
 - [ ] sfence.vma must clear MMU's TLB entries
 - [ ] Can we remove `Raw uint32` from `Instr`?

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

# Appendix

## Emulator Example

``` shell
$ cd cmd/emulator/
$ file examples/hello
examples/hello: ELF 64-bit LSB executable, UCB RISC-V, soft-float ABI, version 1 (SYSV), statically linked, not stripped
$ ./emulator -ktrace examples/hello
    0     0 CALL write(1,10050,15)
Hello, RISC-V!
    0     0 RET  write 15
    0     0 CALL exit(0)
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
