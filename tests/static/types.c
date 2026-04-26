/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#include <sys/types.h>
#include <sys/stat.h>
#include <unistd.h>
#include <stdio.h>
#include <stddef.h>
#include <fcntl.h>
#include <stdlib.h>

static void
stat_fd(int fd)
{
  int ret;
  struct stat stat_st;

  ret = fstat(fd, &stat_st);
  if (ret < 0)
    {
      perror("fstat");
      return;
    }

  /* st_mode: 1ed vs 81ed */
  /* S_IFMT 0170000           - type of file */
  /*        S_IFIFO  0010000  - named pipe (fifo) */
  /*        S_IFCHR  0020000  - character special */
  /*        S_IFDIR  0040000  - directory */
  /*        S_IFBLK  0060000  - block special */
  /*        S_IFREG  0100000  - regular */
  /*        S_IFLNK  0120000  - symbolic link */
  /*        S_IFSOCK 0140000  - socket */
  /*        S_IFWHT  0160000  - whiteout */

  /* 0b1000000000000000*/
  /* 0b0000000111101101*/
  /* 0b1000000111101101*/

  printf(" - st_dev    : %ld\n", stat_st.st_dev);
  printf(" - st_ino    : %ld\n", stat_st.st_ino);
  printf(" - st_mode   : %x\n", stat_st.st_mode);
  printf(" - st_nlink  : %d\n", stat_st.st_nlink);
  printf(" - st_uid    : %d\n", stat_st.st_uid);
  printf(" - st_gid    : %d\n", stat_st.st_gid);
  printf(" - st_rdev   : %ld\n", stat_st.st_rdev);
  printf(" - st_size   : %ld\n", stat_st.st_size);
  printf(" - st_blksize: %d\n", stat_st.st_blksize);
  printf(" - st_blocks : %ld\n", stat_st.st_blocks);
  printf(" - st_atime  : %ld\n", stat_st.st_atime);
  printf(" - st_mtime  : %ld\n", stat_st.st_mtime);
  printf(" - st_ctime  : %ld\n", stat_st.st_ctime);
}

static void
process_file(char *name)
{
  int fd;

  fd = open(name, O_RDONLY);
  if (fd < 0)
    {
      perror("open");
      exit(1);
    }

  printf("fstat(%s)\n", name);
  stat_fd(fd);

  close(fd);

  printf("fstat(1)\n");
  stat_fd(1);
}

int
main(int argc, char *argv[], char *env[])
{
  int i;
  struct stat stat_st;

  printf("sizeof(struct stat)=%ld\n", sizeof(stat_st));
  printf(" - st_dev    : %ld\n", offsetof(struct stat, st_dev));
  printf(" - st_ino    : %ld\n", offsetof(struct stat, st_ino));
  printf(" - st_mode   : %ld\n", offsetof(struct stat, st_mode));
  printf(" - st_nlink  : %ld\n", offsetof(struct stat, st_nlink));
  printf(" - st_uid    : %ld\n", offsetof(struct stat, st_uid));
  printf(" - st_gid    : %ld\n", offsetof(struct stat, st_gid));
  printf(" - st_rdev   : %ld\n", offsetof(struct stat, st_rdev));
  printf(" - st_size   : %ld\n", offsetof(struct stat, st_size));
  printf(" - st_blksize: %ld\n", offsetof(struct stat, st_blksize));
  printf(" - st_blocks : %ld\n", offsetof(struct stat, st_blocks));
  printf(" - st_atime  : %ld\n", offsetof(struct stat, st_atime));
  printf(" - st_mtime  : %ld\n", offsetof(struct stat, st_mtime));
  printf(" - st_ctime  : %ld\n", offsetof(struct stat, st_ctime));

  for (i = 1; i < argc; i++)
    process_file(argv[i]);

  return 0;
}
