.section .text
.global _start

_start:
        ld      a0,0(sp)
        addi    a1,sp,8

        call main

        # return value in a0
        li a7, 93      # SYS_exit
        ecall
