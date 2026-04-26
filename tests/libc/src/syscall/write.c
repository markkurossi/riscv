/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#include "syscall.h"
#include "unistd.h"

ssize_t
write(int fd, const void *buf, size_t count)
{
  return syscall3(SYS_write, fd, (long) buf, count);
}
