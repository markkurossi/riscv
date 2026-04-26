/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */


#include <unistd.h>

const char msg[] = "Hello, RISC-V!\n";

int
main()
{
  write(1, msg, sizeof(msg) - 1);

  return 0;
}
