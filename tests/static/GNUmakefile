
RISCV64_LINUX_CC = riscv64-linux-gnu-gcc

PROG_SRCS := $(wildcard *.c)
PROGS := $(patsubst %.c,%-riscv64-linux,$(PROG_SRCS))

all: $(PROGS)

main-riscv64-linux: main.c
	$(RISCV64_LINUX_CC) -Wall -static -o $@ $+

types-riscv64-linux: types.c
	$(RISCV64_LINUX_CC) -Wall -static -o $@ $+
