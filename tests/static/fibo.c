/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#include <stdio.h>
#include <unistd.h>

unsigned long
fibo(unsigned long n)
{
  if (n <= 1)
    return n;

  return fibo(n - 1) + fibo(n - 2);
}

int
main(int argc, char *argv[], char *env[])
{
  unsigned long n;

  switch (argc)
    {
    case 3:
      n = 40;
      break;

    case 2:
      n = 35;
      break;

    default:
      n = 30;
      break;
    }

  printf("fibo(%ld): %ld\n", n, fibo(n));

  return 0;
}
