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
  return reboot(0xfee1dead, 0x28121969, 0x4321fedc, 0);
}
