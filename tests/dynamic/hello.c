/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#include <stdio.h>
#include <unistd.h>

int
main(int argc, char *argv[], char *env[])
{
  int i;

  printf("Hello, RISC-V!\n");

  for (i = 0; i < argc; i++)
    printf("argv[%d]:\t%s\n", i, argv[i]);

  for (i = 0; env[i]; i++)
    printf("env[%d]:\t%s\n", i, env[i]);

  return 0;
}
