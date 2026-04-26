
all:
	@echo "targets: build, run"

build:
	docker build -t riscv-toolchain .

run:
	docker run -v $(CURDIR):/workspace -it riscv-toolchain

RISCVCC = riscv64-unknown-elf-gcc
RISCVCFLAGS = -march=rv64im -mabi=lp64 -nostdlib -ffreestanding -O2	\
-fno-stack-protector -fno-asynchronous-unwind-tables -fno-pic

INCLUDES := -Iephemelier
LIBCSRCS := $(wildcard ephemelier/*.c)

world: world.c
	$(RISCVCC) $(RISCVCFLAGS) $(INCLUDES) -o $@ $+ $(LIBCSRCS)
