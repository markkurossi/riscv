/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#include <stdio.h>
#include <stdlib.h>
#include <time.h>
#include <spawn.h>
#include <sys/wait.h>
#include <sys/resource.h>

#ifdef __riscv
#define riscv_read_csr(csr) ({                               \
    unsigned long __v;                                       \
    __asm__ __volatile__ ("csrr %0, " #csr : "=r" (__v));    \
    __v;                                                     \
})

#define time_ns() riscv_read_csr(0x7c1)

#else  /* not __riscv */
#define riscv_read_csr(csr) 0

unsigned long
time_ns()
{
  struct timespec ts;

  clock_gettime(CLOCK_MONOTONIC, &ts);

  return ts.tv_sec * 1000000000 + ts.tv_nsec;
}

#endif  /* not __riscv */

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

  if (argc == 1)
    {
      fprintf(stderr, "Usage: time program [args...]\n");
      return 1;
    }

  start = time_ns();
  cycle = riscv_read_csr(cycle);
  instret = riscv_read_csr(instret);
  status = posix_spawnp(&pid, argv[1], NULL, NULL, argv+1, environ);
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
