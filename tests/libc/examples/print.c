/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#include <unistd.h>
#include <intconv.h>
#include <string.h>

int
main()
{
  char buf[64];
  int len;

  itoa(12345, buf, sizeof(buf), 10);

  len = strlen(buf);
  buf[len++] = '\n';

  write(1, buf, len);

  return 0;
}
