/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#include <stdio.h>
#include <unistd.h>
#include <sys/reboot.h>
#include <linux/reboot.h>

int
main(int argc, char *argv[], char *env[])
{
  int result = reboot(LINUX_REBOOT_CMD_POWER_OFF);

  if (result < 0)
    perror("reboot failed");

  return result;
}
