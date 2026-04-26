/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#include <unistd.h>
#include <intconv.h>
#include <string.h>

int
main(int argc, char *argv[])
{
  char buf[64];
  int i;
  struct iovec iovec[2];

  itoa(argc, buf, sizeof(buf), 10);

  iovec[0].iov_base = buf;
  iovec[0].iov_len = strlen(buf);

  iovec[1].iov_base = "\n";
  iovec[1].iov_len = 1;

  writev(1, iovec, 2);

  for (i = 0; i < argc; i++)
    {
      iovec[0].iov_base = argv[i];
      iovec[0].iov_len = strlen(argv[i]);
      writev(1, iovec, 2);
    }

  return 0;
}
