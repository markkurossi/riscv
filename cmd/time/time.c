/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <time.h>
#include <spawn.h>
#include <sys/wait.h>
#include <sys/resource.h>

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

void print_riscv_extensions();

int
main(int argc, char *argv[])
{
  pid_t pid;
  extern char **environ;
  int status;
  struct rusage rusage;
  unsigned long instret, cycle;
  unsigned long start, end;
  double elapsed;
  int i;
  bool cpu_profile = false;

  for (i = 1; i < argc; i++)
    {
      if (strcmp(argv[i], "-cpu") == 0)
        cpu_profile = true;
      else
        break;
    }
  if (i < argc && strcmp(argv[i], "info") == 0)
    {
      cycle = riscv_read_csr(cycle);
      instret = riscv_read_csr(instret);
      printf("CPU info:\n");
      printf(" - cycles : %ld\n", cycle);
      printf(" - instret: %ld\n", instret);

      print_riscv_extensions();

      return 0;
    }
  if (i >= argc)
    {
      fprintf(stderr, "Usage: time program [args...]\n");
      fprintf(stderr, "       time info\n");
      return 1;
    }

  if (cpu_profile)
    riscv_write_csr(0x802, 1);

  start = time_ns();
  cycle = riscv_read_csr(cycle);
  instret = riscv_read_csr(instret);
  status = posix_spawnp(&pid, argv[i], NULL, NULL, argv+i, environ);
  if (status == 0)
    {
      waitpid(pid, &status, 0);
    }
  else
    {
      perror("posix_spawn failed");
      return 1;
    }
  cycle = riscv_read_csr(cycle) - cycle;
  instret = riscv_read_csr(instret) - instret;
  end = time_ns();

  if (cpu_profile)
    riscv_write_csr(0x802, 0);

  elapsed = ((double) end - (double) start) * 1e-9;

  status = getrusage(RUSAGE_CHILDREN, &rusage);
  if (status == -1)
    {
      perror("getrusage");
      return 1;
    }
  printf("%15.6f real %9ld.%06ld user %9ld.%06ld sys\n",
         elapsed,
         rusage.ru_utime.tv_sec, (long) rusage.ru_utime.tv_usec,
         rusage.ru_stime.tv_sec, (long) rusage.ru_stime.tv_usec);
  printf("%20ld maximum resident set size\n",    rusage.ru_maxrss);
  printf("%20ld average shared memory size\n",   rusage.ru_ixrss);
  printf("%20ld average unshared data size\n",   rusage.ru_idrss);
  printf("%20ld average unshared stack size\n",  rusage.ru_isrss);
  printf("%20ld page reclaims\n",                rusage.ru_minflt);
  printf("%20ld page faults\n",                  rusage.ru_majflt);
  printf("%20ld swaps\n",                        rusage.ru_nswap);
  printf("%20ld block input operations\n",       rusage.ru_inblock);
  printf("%20ld block output operations\n",      rusage.ru_oublock);
  printf("%20ld messages sent\n",                rusage.ru_msgsnd);
  printf("%20ld messages received\n",            rusage.ru_msgrcv);
  printf("%20ld signals received\n",             rusage.ru_nsignals);
  printf("%20ld voluntary context switches\n",   rusage.ru_nvcsw);
  printf("%20ld involuntary context switches\n", rusage.ru_nivcsw);
  printf("%20ld instructions retired\n",         instret);
  printf("%20ld cycles elapsed\n",               cycle);
  printf("%20.2f MIPS\n", (double) instret / 1000000.0 / elapsed);

  return 0;
}
