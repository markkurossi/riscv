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
