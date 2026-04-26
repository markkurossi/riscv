/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#include <stddef.h>

void *
memcpy(void *dst, const void *src, size_t n)
{
  char *d = dst;
  const char *s = src;

  for (size_t i = 0; i < n; i++)
    d[i] = s[i];

  return dst;
}
