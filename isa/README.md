

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
