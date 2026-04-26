/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#include <stddef.h>
#include <intconv.h>

char *
itoa(int value, char *buf, size_t size, int __attribute__((unused)) base)
{
  size_t i, ofs;
  int neg;

  if (value < 0)
    {
      neg = 1;
      value = -value;
    }
  else
    {
      neg = 0;
    }

  for (ofs = 0; ofs < size-1; )
    {
      buf[ofs++] = '0' + (value % 10);
      value /= 10;
      if (value == 0)
        break;
    }
  if (ofs < size-1 && neg)
    buf[ofs++] = '-';
  buf[ofs] = '\0';

  /* Reverse result. */
  for (i = 0; i < ofs / 2; i++)
    {
      char tmp = buf[i];

      buf[i] = buf[ofs - 1 - i];
      buf[ofs - 1 - i] = tmp;
    }

  return buf;
}
