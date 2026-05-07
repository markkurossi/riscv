/*
 * uname.c
 */

#include <stdio.h>
#include <stdlib.h>
#include <sys/utsname.h>

int
main()
{
  // Define the structure to hold system information
  struct utsname buffer;

  // uname returns 0 on success, -1 on failure
  if (uname(&buffer) != 0)
    {
      perror("uname");
      exit(EXIT_FAILURE);
    }

  printf("System Information:\n");
  printf("-------------------\n");
  printf("Operating System: %s\n", buffer.sysname);
  printf("Node Name (Host): %s\n", buffer.nodename);
  printf("Release:          %s\n", buffer.release);
  printf("Version:          %s\n", buffer.version);
  printf("Machine:          %s\n", buffer.machine);

#ifdef _GNU_SOURCE
  printf("Domain Name:      %s\n", buffer.domainname);
#endif

  return 0;
}
