.section .text
.global _start

_start:
    # a0 = argc
    # a1 = argv

    call main

    # return value in a0
    li a7, 93      # SYS_exit
    ecall
