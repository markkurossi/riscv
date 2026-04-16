# RISC-V in Go

## Emulator Example

``` shell
$ cd cmd/emulator/
$ ./emulator bin/hello
   1002c:   fd5ff0ef    jal     -2c
   10000:   000105b7    lui     a1,0x10
   10004:   ff010113    addi    sp,sp,-16
   10008:   00f00613    addi    a2,zero,15
   1000c:   04858593    addi    a1,a1,72
   10010:   00100513    addi    a0,zero,1
   10014:   00113423    sd      ra,8(sp)
   10018:   020000ef    jal     20
   10038:   04000893    addi    a7,zero,64
   1003c:   00000073    ecall
ecall: write(1,65608,15)
Hello, RISC-V!
   10040:   00008067    jalr    zero,0(ra)
   1001c:   00813083    ld      ra,8(sp)
   10020:   00000513    addi    a0,zero,0
   10024:   01010113    addi    sp,sp,16
   10028:   00008067    jalr    zero,0(ra)
   10030:   05d00893    addi    a7,zero,93
   10034:   00000073    ecall
ecall: exit(0)
```
