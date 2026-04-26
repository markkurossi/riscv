/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#include "syscall.h"
#include "unistd.h"

ssize_t
read(int fd, void *buf, size_t count)
{
  return syscall3(SYS_read, fd, (long) buf, count);
}
