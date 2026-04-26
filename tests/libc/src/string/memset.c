/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#include <stddef.h>

void *
memset(void *dst, int c, size_t n)
{
  unsigned char *d = dst;

  for (size_t i = 0; i < n; i++)
    d[i] = (unsigned char)c;

  return dst;
}
