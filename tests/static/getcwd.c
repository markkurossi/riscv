/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#include <stdio.h>
#include <unistd.h>

int
main(int argc, char *argv[], char *env[])
{
  char buf[256];
  char *cwd;

  cwd = getcwd(buf, sizeof(buf));
  if (cwd == NULL)
    {
      perror("getcwd");
      return 1;
    }

  printf("cwd: %s\n", cwd);

  return 0;
}
