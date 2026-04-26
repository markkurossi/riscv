/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#ifndef _UNISTD_H_
#define _UNISTD_H_

#include <stddef.h>

ssize_t read(int fd, void *buf, size_t count);
ssize_t write(int fd, const void *buf, size_t count);
void _exit(int status);

#endif /* not _UNISTD_H_ */
