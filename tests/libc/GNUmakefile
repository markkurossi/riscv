CC = riscv64-unknown-elf-gcc
AR = riscv64-unknown-elf-ar
OBJDUMP = riscv64-unknown-elf-objdump

CFLAGS = -march=rv64im_zkne -mabi=lp64 -nostdlib -ffreestanding -fno-builtin -O2 -Wall -Wextra -Iinclude
LDFLAGS = -nostdlib -T linker/linker.ld

LIBCSRC = $(wildcard src/string/*.c) $(wildcard src/syscall/*.c)	\
$(wildcard src/intconv/*.c)

OBJ = $(patsubst %.c,%.o,$(LIBCSRC))

all: libc.a crt0.o

libc.a: $(OBJ)
	$(AR) rcs $@ $^

crt0.o:
	$(CC) $(CFLAGS) -c src/crt/crt0.S -o $@

encrypt.o:
	$(CC) $(CFLAGS) -c src/aes/encrypt.S -o $@

examples/hello: examples/hello.c libc.a crt0.o
	$(CC) $(CFLAGS) examples/hello.c crt0.o -L. -lc $(LDFLAGS) -o $@

examples/print: examples/print.c libc.a crt0.o
	$(CC) $(CFLAGS) examples/print.c crt0.o -L. -lc $(LDFLAGS) -o $@

run: examples/hello
	qemu-riscv64 examples/hello

objdump:
	$(OBJDUMP) -d examples/hello

clean:
	rm -f $(OBJ) libc.a crt0.o examples/hello
