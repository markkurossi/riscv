/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#if defined(__linux__)

#include <sys/auxv.h>
#include <unistd.h>
#include <sys/syscall.h>

#ifndef __NR_riscv_hwprobe
#define __NR_riscv_hwprobe 258
#endif

struct riscv_hwprobe
{
  long long key;
  unsigned long long value;
};

#define RISCV_HWPROBE_KEY_IMA_EXT_0 4

// Bit mapping for Linux hwprobe (up to v6.8).
struct hwprobe_ext
{
  unsigned long long mask;
  const char *name;
};

static const struct hwprobe_ext hwprobe_exts[] =
  {
    { 1ULL << 0,  "fd" },
    { 1ULL << 1,  "c" },
    { 1ULL << 2,  "v" },
    { 1ULL << 3,  "zba" },
    { 1ULL << 4,  "zbb" },
    { 1ULL << 5,  "zbs" },
    { 1ULL << 6,  "zicboz" },
    { 1ULL << 7,  "zbc" },
    { 1ULL << 8,  "zbkb" },
    { 1ULL << 9,  "zbkc" },
    { 1ULL << 10, "zbkx" },
    { 1ULL << 11, "zknd" },
    { 1ULL << 12, "zkne" },
    { 1ULL << 13, "zknh" },
    { 1ULL << 14, "zksed" },
    { 1ULL << 15, "zksh" },
    { 1ULL << 16, "zkt" },
    { 1ULL << 17, "zvbb" },
    { 1ULL << 18, "zvbc" },
    { 1ULL << 19, "zvkg" },
    { 1ULL << 20, "zvkned" },
    { 1ULL << 21, "zvknha" },
    { 1ULL << 22, "zvknhb" },
    { 1ULL << 23, "zvksed" },
    { 1ULL << 24, "zvksh" },
    { 1ULL << 25, "zvkt" },
    { 1ULL << 26, "zfh" },
    { 1ULL << 27, "zfhmin" },
    { 1ULL << 28, "zihintntl" },
    { 1ULL << 29, "zvfh" },
    { 1ULL << 30, "zvfhmin" },
    { 1ULL << 31, "zfa" },
    { 1ULL << 32, "ztso" },
    { 1ULL << 33, "zacas" },
    { 1ULL << 34, "zicond" },
    { 1ULL << 35, "zihintpause" },
};

#elif defined(__FreeBSD__)

#include <sys/auxv.h>
#include <sys/types.h>
#include <sys/sysctl.h>

#ifndef AT_HWCAP
#define AT_HWCAP 25
#endif

#else
// NetBSD and macOS do not implement <sys/auxv.h> or a standardized
// RISC-V userland interface yet.
#endif

void
print_riscv_extensions()
{
#if !defined(__linux__) && !defined(__FreeBSD__)
  printf(" - base   : rv64\n");
  printf(" - single : [API not available on this OS]\n");
  printf(" - z-exts : [API not available on this OS]\n");
  return;
#else  /* __linux__ || __FreeBSD__ */
  unsigned long hwcap = 0;

  // Fetch the single-letter AT_HWCAP bitmask.
#if defined(__linux__)
  hwcap = getauxval(AT_HWCAP);
#elif defined(__FreeBSD__)
  elf_aux_info(AT_HWCAP, &hwcap, sizeof(hwcap));
#endif

  // The official RISC-V canonical ordering for base extensions.
  const char canonical[] = "imafdqcbvkjphs";

  printf(" - base   : rv64");
  unsigned long temp_hwcap = hwcap;

  // Extract according to canonical order
  for (int i = 0; canonical[i] != '\0'; i++)
    {
      int bit = canonical[i] - 'a';
      if (temp_hwcap & (1UL << bit))
        {
          printf("%c", canonical[i]);
          temp_hwcap &= ~(1UL << bit);
        }
    }

  // Catch any remaining set bits that fall outside standard canonical
  // letters.
  for (int i = 0; i < 26; i++) {
    if (temp_hwcap & (1UL << i))
      printf("%c", 'a' + i);
  }
  printf("\n");

  printf(" - single : ");
  int first_single = 1;
  temp_hwcap = hwcap;

  for (int i = 0; canonical[i] != '\0'; i++)
    {
      int bit = canonical[i] - 'a';
      if (temp_hwcap & (1UL << bit))
        {
          printf("%s%c", first_single ? "" : ", ", canonical[i]);
          first_single = 0;
          temp_hwcap &= ~(1UL << bit);
        }
    }
  for (int i = 0; i < 26; i++)
    {
      if (temp_hwcap & (1UL << i))
        {
          printf("%s%c", first_single ? "" : ", ", 'a' + i);
          first_single = 0;
        }
    }
  if (first_single)
    printf("none");
  printf("\n");

  /* Z and V Extensions. */
#if defined(__linux__)
  struct riscv_hwprobe pairs[1];
  pairs[0].key = RISCV_HWPROBE_KEY_IMA_EXT_0;
  pairs[0].value = 0;

  if (syscall(__NR_riscv_hwprobe, pairs, 1, 0, NULL, 0) == 0)
    {
      printf(" - z-exts : ");
      int first_z = 1;
      for (size_t i = 0; i < sizeof(hwprobe_exts)/sizeof(hwprobe_exts[0]); i++)
        {
          if (pairs[0].value & hwprobe_exts[i].mask)
            {
              printf("%s%s", first_z ? "" : ", ", hwprobe_exts[i].name);
              first_z = 0;
            }
        }
      if (first_z)
        printf("none");
      printf("\n");
    }
  else
    {
      printf(" - z-exts : [hwprobe syscall failed]\n");
    }

#elif defined(__FreeBSD__)
  const char* sysctl_keys[] = {"hw.cpu_isa", "hw.isa", NULL};
  int found_isa = 0;

  for (int i = 0; sysctl_keys[i] != NULL; i++)
    {
      char isa_str[512] = {0};
      size_t len = sizeof(isa_str) - 1;

      if (sysctlbyname(sysctl_keys[i], isa_str, &len, NULL, 0) == 0)
        {
          found_isa = 1;
          char *exts = strchr(isa_str, '_'); // Skip past the base string

          if (exts)
            {
              printf(" - z-exts : ");
              exts++;

              // Format nicely: comma-delimited and lowercase
              for (char *p = exts; *p; p++)
                {
                  if (*p == '_')
                    *p = ',';
                  else if (*p >= 'A' && *p <= 'Z')
                    *p += 32;
                }
              for (char *p = exts; *p; p++)
                {
                  putchar(*p);
                  if (*p == ',')
                    putchar(' ');
                }
              printf("\n");
            }
          else
            {
              printf(" - z-exts : none\n");
            }
          break;
        }
    }

  if (!found_isa)
    printf(" - z-exts : [could not read sysctl hw.isa]\n");
#endif /* __FreeBSD__ */
#endif /* __linux__ || __FreeBSD__ */
}
