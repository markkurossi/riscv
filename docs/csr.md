# Control and Status Registers

## Instructions

| Instr     | Set CSR To | Arg {} | Arg {i}     | atomic.Uint64        |
|-----------|------------|--------|-------------|----------------------|
| csrrc{,i} | csr & ^arg | X[rs1] | uint64(rs1) | return csr.And(^arg) |
| csrrs{,i} | csr \| arg | X[rs1] | uint64(rs1) | return csr.Or(arg)   |
| csrrw{,i} | arg        | X[rs1] | uint64(rs1) | return csr.Swap(arg) |

If arg is zero (`isa.Zero`), all operations are read-only.
