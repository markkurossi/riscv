/*
 * mprotect.c
 */

#include <stdio.h>
#include <string.h>
#include <stdlib.h>
#include <sys/mman.h>

int
main()
{
  void *data;
  size_t page_size = 4096;

  data = mmap(0, 2 * page_size, PROT_READ|PROT_WRITE,
              MAP_ANON | MAP_PRIVATE, -1, 0);
  if (data == MAP_FAILED)
    {
      perror("mmap");
      return 1;
    }

#if 1
  printf("Clearning data:\n");
  memset(data, 0, 2 * page_size);
#endif

  printf("Protecting data...\n");
  if (mprotect(data + page_size, page_size, PROT_NONE) == -1)
    {
      perror("mprotect");
      return 1;
    }

  printf("Setting data:\n");
  fflush(NULL);
  memset(data, 0, 2 * page_size);

  printf("done\n");

  return 0;
}
