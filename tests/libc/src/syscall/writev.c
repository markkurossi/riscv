/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#include "syscall.h"
#include "unistd.h"

ssize_t
writev(int fd, const struct iovec *iov, int iovcnt)
{
  return syscall3(SYS_writev, fd, (long) iov, iovcnt);
}
