/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#ifndef GOEMU_H
#define GOEMU_H

#include <time.h>

#ifdef __riscv
/*
 * Read a RISC-V CSR into an unsigned long variable.
 * Usage: unsigned long val = riscv_read_csr(0x802);
 */
#define riscv_read_csr(csr) ({                                  \
    unsigned long __v;                                          \
    __asm__ __volatile__ ("csrr %0, " #csr : "=r" (__v));       \
    __v;                                                        \
})

/*
 * Write an unsigned long value to a RISC-V CSR.
 * Usage: riscv_write_csr(0x802, v);
 */
#define riscv_write_csr(csr, val)                               \
do {                                                            \
  unsigned long __v = (unsigned long)(val);                     \
  __asm__ __volatile__ ("csrw " #csr ", %0" : : "r" (__v));     \
 } while (0)

#define time_ns() riscv_read_csr(0x801)

#else  /* not __riscv */

#define riscv_read_csr(csr) 0
#define riscv_write_csr(csr, val)

unsigned long
time_ns()
{
  struct timespec ts;

  clock_gettime(CLOCK_MONOTONIC, &ts);

  return ts.tv_sec * 1000000000 + ts.tv_nsec;
}

#endif  /* not __riscv */

#endif /* not GOEMU_H */
