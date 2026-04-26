/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#include <stddef.h>

size_t
strlen(const char *s)
{
  size_t n = 0;

  while (s[n])
    n++;

  return n;
}
