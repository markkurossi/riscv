/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#include "syscall.h"
#include "unistd.h"

void
_exit(int status)
{
  syscall1(SYS_exit, status);

  // Should never return.
  while (1)
    ;
}
