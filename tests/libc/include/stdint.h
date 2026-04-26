/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#ifndef _STDINT_H_
#define _STDINT_H_

typedef signed char int8_t;
typedef short int16_t;
typedef int int32_t;

typedef unsigned char uint8_t;
typedef unsigned short uint16_t;
typedef unsigned int uint32_t;

#if __SIZEOF_POINTER__ == 8

typedef long int64_t;
typedef unsigned long uint64_t;

#elif __SIZEOF_POINTER__ == 4

typedef long long int64_t;
typedef unsigned long long uint64_t;

#else
#error "Unknown pointer size"
#endif

#endif /* not _STDINT_H_ */
