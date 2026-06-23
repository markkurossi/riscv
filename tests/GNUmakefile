
all:
	@echo "targets: build, run"

build:
	docker build -t riscv-toolchain .

run:
	docker run -v $(CURDIR):/workspace -it riscv-toolchain

run-image:
	docker run --privileged \
	-v $(CURDIR):/workspace \
	-v $(CURDIR)/../cmd/goemu/ubuntu-24.04/ubuntu-24.04.4-preinstalled-server-riscv64.img:/image.ext2 \
	-it riscv-toolchain

RISCVCC = riscv64-unknown-elf-gcc
RISCVCFLAGS = -march=rv64im -mabi=lp64 -nostdlib -ffreestanding -O2	\
-fno-stack-protector -fno-asynchronous-unwind-tables -fno-pic

INCLUDES := -Iephemelier
LIBCSRCS := $(wildcard ephemelier/*.c)

world: world.c
	$(RISCVCC) $(RISCVCFLAGS) $(INCLUDES) -o $@ $+ $(LIBCSRCS)

%.dts : %.dtb
	dtc -I dtb -O dts -o $@ $+
