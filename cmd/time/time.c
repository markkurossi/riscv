/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#include <stdio.h>
#include <stdlib.h>
#include <spawn.h>
#include <sys/wait.h>
#include <sys/resource.h>

#ifdef __riscv
#define riscv_read_csr(csr) ({                               \
    unsigned long __v;                                       \
    __asm__ __volatile__ ("csrr %0, " #csr : "=r" (__v));    \
    __v;                                                     \
})
#else  /* not __riscv */
#define riscv_read_csr(csr) 0
#endif  /* not __riscv */

int
main(int argc, char *argv[])
{
  pid_t pid;
  extern char **environ;
  int status;
  struct rusage rusage;
  unsigned long instret, cycle;

  if (argc == 1)
    {
      fprintf(stderr, "Usage: time program [args...]\n");
      return 1;
    }

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

  status = getrusage(RUSAGE_CHILDREN, &rusage);
  if (status == -1)
    {
      perror("getrusage");
      return 1;
    }
  printf("%15ld.%06d user %9ld.%06d sys\n",
    rusage.ru_utime.tv_sec, rusage.ru_utime.tv_usec,
    rusage.ru_stime.tv_sec, rusage.ru_stime.tv_usec);
  printf("%16ld maximum resident set size\n",    rusage.ru_maxrss);
  printf("%16ld average shared memory size\n",   rusage.ru_ixrss);
  printf("%16ld average unshared data size\n",   rusage.ru_idrss);
  printf("%16ld average unshared stack size\n",  rusage.ru_isrss);
  printf("%16ld page reclaims\n",                rusage.ru_minflt);
  printf("%16ld page faults\n",                  rusage.ru_majflt);
  printf("%16ld swaps\n",                        rusage.ru_nswap);
  printf("%16ld block input operations\n",       rusage.ru_inblock);
  printf("%16ld block output operations\n",      rusage.ru_oublock);
  printf("%16ld messages sent\n",                rusage.ru_msgsnd);
  printf("%16ld messages received\n",            rusage.ru_msgrcv);
  printf("%16ld signals received\n",             rusage.ru_nsignals);
  printf("%16ld voluntary context switches\n",   rusage.ru_nvcsw);
  printf("%16ld involuntary context switches\n", rusage.ru_nivcsw);
  printf("%16ld instructions retired\n",         instret);
  printf("%16ld cycles elapsed\n",               cycle);

  return 0;
}
