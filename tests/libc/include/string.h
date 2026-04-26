/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#ifndef _STRING_H_
#define _STRING_H_

#include <stddef.h>

size_t strlen(const char *s);

size_t strnlen(const char *s, size_t maxlen);

#endif /* not _STRING_H_ */
