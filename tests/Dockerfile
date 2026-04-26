FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

# Install prebuilt RISC-V toolchains + QEMU
RUN apt-get update
RUN apt-get install -y gcc-riscv64-unknown-elf
RUN apt-get install -y binutils-riscv64-unknown-elf
RUN apt-get install -y gcc-riscv64-linux-gnu
RUN apt-get install -y libc6-dev-riscv64-cross
RUN apt-get install -y qemu-user
RUN apt-get install -y qemu-user-static
RUN apt-get install -y qemu-system-misc
RUN apt-get install -y gdb-multiarch
RUN apt-get install -y make
RUN apt-get install -y git
RUN apt-get install -y ca-certificates
RUN apt-get install -y build-essential
RUN apt-get install -y file
RUN apt-get install -y wget
RUN apt-get install -y cpio
RUN apt-get install -y unzip
RUN apt-get install -y rsync
RUN apt-get install -y bc
RUN apt-get install -y flex
RUN apt-get update
RUN apt-get install -y libncurses-dev
RUN apt-get install -y bison

#RUN rm -rf /var/lib/apt/lists/*

WORKDIR /workspace

# Default MPC-friendly flags
ENV RISCV_MPC_FLAGS="-march=rv64im -mabi=lp64 -nostdlib -ffreestanding -O2"
ENV RISCV_LINUX_FLAGS="-march=rv64gc -mabi=lp64d -O2"

# Verify installations
RUN riscv64-unknown-elf-gcc --version && \
    riscv64-linux-gnu-gcc --version && \
    qemu-riscv64 --version
