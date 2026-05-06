
RISCV64_LINUX_CC = riscv64-linux-gnu-gcc

PROG_SRCS := $(wildcard *.c)

RISCV_PROGS := $(patsubst %.c,riscv64-linux/%,$(PROG_SRCS))
HOST_PROGS := $(patsubst %.c,host/%,$(PROG_SRCS))

ALL_TARGETS :=

UNAME_S := $(shell uname -s)

ifeq ($(UNAME_S),Darwin)
ALL_TARGETS += $(HOST_PROGS)
else
ALL_TARGETS += $(RISCV_PROGS)
endif

all: $(ALL_TARGETS)

riscv64-linux/%: %.c
	$(RISCV64_LINUX_CC) -Wall -static -o $@ $+

host/%: %.c
	$(CC) -Wall -o $@ $+

clean:
	$(RM) -f $(HOST_PROGS) $(RISCV_PROGS)
