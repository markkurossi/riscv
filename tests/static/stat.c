/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#include <sys/stat.h>
#include <stdio.h>

int
main(int argc, char *argv[])
{
  int i, ret;
  struct stat stat_st;

  for (i = 1; i < argc; i++)
    {
      ret = stat(argv[i], &stat_st);
      if (ret == -1)
        {
          perror("stat");
          return 1;
        }
      printf("%s:\n", argv[i]);
      printf(" - size: %ld\n", (long) stat_st.st_size);
    }

  return 0;
}
