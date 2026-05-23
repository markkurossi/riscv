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
  char *data = 0;

  printf("data[%d] = %d\n", 42, data[42]);

  return 0;
}
