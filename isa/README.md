
## hello

``` shell
root@7de3839c53b7:/workspace/libc/examples# riscv64-unknown-elf-objdump -d hello

hello:     file format elf64-littleriscv

Disassembly of section .text:

0000000000010000 <main>:
   10000:	000105b7          	lui	a1,0x10
   10004:	ff010113          	addi	sp,sp,-16
   10008:	00f00613          	li	a2,15
   1000c:	04858593          	addi	a1,a1,72 # 10048 <msg>
   10010:	00100513          	li	a0,1
   10014:	00113423          	sd	ra,8(sp)
   10018:	020000ef          	jal	10038 <write>
   1001c:	00813083          	ld	ra,8(sp)
   10020:	00000513          	li	a0,0
   10024:	01010113          	addi	sp,sp,16
   10028:	00008067          	ret

000000000001002c <_start>:
   1002c:	fd5ff0ef          	jal	10000 <main>
   10030:	05d00893          	li	a7,93
   10034:	00000073          	ecall

0000000000010038 <write>:
   10038:	04000893          	li	a7,64
   1003c:	00000073          	ecall
   10040:	00008067          	ret
```

## itoa

``` c
/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#include <unistd.h>
#include <intconv.h>
#include <string.h>

int
main()
{
  char buf[64];

  itoa(12345, buf, sizeof(buf), 10);

  write(1, buf, strlen(buf));

  buf[0] = '\n';
  write(1, buf, 1);

  return 0;
}
```

``` c
/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#include <stddef.h>
#include <intconv.h>

char *
itoa(int value, char *buf, size_t size, int __attribute__((unused)) base)
{
  size_t i, ofs;
  int neg;

  if (value < 0)
    {
      neg = 1;
      value = -value;
    }
  else
    {
      neg = 0;
    }

  for (ofs = 0; ofs < size-1; )
    {
      buf[ofs++] = '0' + (value % 10);
      value /= 10;
      if (value == 0)
        break;
    }
  if (ofs < size-1 && neg)
    buf[ofs++] = '-';
  buf[ofs] = '\0';

  /* Reverse result. */
  for (i = 0; i < ofs / 2; i++)
    {
      char tmp = buf[i];

      buf[i] = buf[ofs - 1 - i];
      buf[ofs - 1 - i] = tmp;
    }

  return buf;
}

```

``` shell
# riscv64-unknown-elf-objdump -d examples/print

examples/print:     file format elf64-littleriscv


Disassembly of section .text:

0000000000010000 <main>:
   10000:       fb010113                addi    sp,sp,-80
   10004:       00003537                lui     a0,0x3
   10008:       00a00693                li      a3,10
   1000c:       00010593                mv      a1,sp
   10010:       04000613                li      a2,64
   10014:       03950513                addi    a0,a0,57 # 3039 <main-0xcfc7>
   10018:       04113423                sd      ra,72(sp)
   1001c:       084000ef                jal     100a0 <itoa>
   10020:       00010513                mv      a0,sp
   10024:       048000ef                jal     1006c <strlen>
   10028:       00050613                mv      a2,a0
   1002c:       00010593                mv      a1,sp
   10030:       00100513                li      a0,1
   10034:       060000ef                jal     10094 <write>
   10038:       00010593                mv      a1,sp
   1003c:       00a00793                li      a5,10
   10040:       00100613                li      a2,1
   10044:       00100513                li      a0,1
   10048:       00f10023                sb      a5,0(sp)
   1004c:       048000ef                jal     10094 <write>
   10050:       04813083                ld      ra,72(sp)
   10054:       00000513                li      a0,0
   10058:       05010113                addi    sp,sp,80
   1005c:       00008067                ret

0000000000010060 <_start>:
   10060:       fa1ff0ef                jal     10000 <main>
   10064:       05d00893                li      a7,93
   10068:       00000073                ecall

000000000001006c <strlen>:
   1006c:       00054783                lbu     a5,0(a0)
   10070:       00050713                mv      a4,a0
   10074:       00000513                li      a0,0
   10078:       00078c63                beqz    a5,10090 <strlen+0x24>
   1007c:       00150513                addi    a0,a0,1
   10080:       00a707b3                add     a5,a4,a0
   10084:       0007c783                lbu     a5,0(a5)
   10088:       fe079ae3                bnez    a5,1007c <strlen+0x10>
   1008c:       00008067                ret
   10090:       00008067                ret

0000000000010094 <write>:
   10094:       04000893                li      a7,64
   10098:       00000073                ecall
   1009c:       00008067                ret

00000000000100a0 <itoa>:
   100a0:       00050713                mv      a4,a0
   100a4:       00058513                mv      a0,a1
   100a8:       00000593                li      a1,0
   100ac:       00075663                bgez    a4,100b8 <itoa+0x18>
   100b0:       40e0073b                negw    a4,a4
   100b4:       00100593                li      a1,1
   100b8:       fff60793                addi    a5,a2,-1
   100bc:       00a00893                li      a7,10
   100c0:       00000613                li      a2,0
   100c4:       01c0006f                j       100e0 <itoa+0x40>
   100c8:       031766bb                remw    a3,a4,a7
   100cc:       0317473b                divw    a4,a4,a7
   100d0:       0306869b                addiw   a3,a3,48
   100d4:       fed30fa3                sb      a3,-1(t1)
   100d8:       04070a63                beqz    a4,1012c <itoa+0x8c>
   100dc:       00080613                mv      a2,a6
   100e0:       00160813                addi    a6,a2,1
   100e4:       01050333                add     t1,a0,a6
   100e8:       fef610e3                bne     a2,a5,100c8 <itoa+0x28>
   100ec:       00f50733                add     a4,a0,a5
   100f0:       00070023                sb      zero,0(a4)
   100f4:       0017d593                srli    a1,a5,0x1
   100f8:       02058863                beqz    a1,10128 <itoa+0x88>
   100fc:       fff78793                addi    a5,a5,-1
   10100:       00050713                mv      a4,a0
   10104:       00f507b3                add     a5,a0,a5
   10108:       00a585b3                add     a1,a1,a0
   1010c:       0007c603                lbu     a2,0(a5)
   10110:       00074683                lbu     a3,0(a4)
   10114:       00170713                addi    a4,a4,1
   10118:       fff78793                addi    a5,a5,-1
   1011c:       fec70fa3                sb      a2,-1(a4)
   10120:       00d780a3                sb      a3,1(a5)
   10124:       feb714e3                bne     a4,a1,1010c <itoa+0x6c>
   10128:       00008067                ret
   1012c:       02f87263                bgeu    a6,a5,10150 <itoa+0xb0>
   10130:       02058063                beqz    a1,10150 <itoa+0xb0>
   10134:       00260793                addi    a5,a2,2
   10138:       02d00713                li      a4,45
   1013c:       00e30023                sb      a4,0(t1)
   10140:       00f50733                add     a4,a0,a5
   10144:       00070023                sb      zero,0(a4)
   10148:       0017d593                srli    a1,a5,0x1
   1014c:       fb1ff06f                j       100fc <itoa+0x5c>
   10150:       00080793                mv      a5,a6
   10154:       f99ff06f                j       100ec <itoa+0x4c>
```
