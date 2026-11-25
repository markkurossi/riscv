# OpenSBI and Das U-Boot

## OpenSBI


[source](https://github.com/riscv-software-src/opensbi)

``` shell
$ cd opensbi
$ export CROSS_COMPILE=riscv64-linux-gnu-
$ export PLATFORM=generic
$ make
$ make install
```

OpenSBI debug build:

``` shell
$ make DEBUG=1
```

Binaries are at:

``` shell
# ls -l install/usr/share/opensbi/lp64/generic/firmware/
total 9200
-rw-r--r-- 1 root root  277880 Jun 19 08:49 fw_dynamic.bin
-rwxr-xr-x 1 root root 2245904 Jun 19 08:49 fw_dynamic.elf
-rw-r--r-- 1 root root  277840 Jun 19 08:49 fw_jump.bin
-rwxr-xr-x 1 root root 2245480 Jun 19 08:49 fw_jump.elf
-rw-r--r-- 1 root root 2105672 Jun 19 08:49 fw_payload.bin
-rwxr-xr-x 1 root root 2254888 Jun 19 08:49 fw_payload.elf
drwxr-xr-x 4 root root     128 Jun 19 08:49 payloads
```

## Das U-Boot

[source](https://git.u-boot-project.org/u-boot/u-boot/-/releases)

``` shell
$ apt-get update && apt-get install -y libssl-dev
$ apt-get update && apt-get install -y libgnutls28-dev uuid-dev
```

``` shell
$ cd u-boot-2026.04/
$ export CROSS_COMPILE=riscv64-linux-gnu-
$ ls configs/ | grep riscv
openpiton_riscv64_defconfig
openpiton_riscv64_spl_defconfig
qemu-riscv64_defconfig
qemu-riscv64_smode_acpi_defconfig
qemu-riscv64_smode_defconfig
qemu-riscv64_spl_defconfig
$ make qemu-riscv64_smode_defconfig
  HOSTCC  scripts/basic/fixdep
  HOSTCC  scripts/kconfig/conf.o
  YACC    scripts/kconfig/zconf.tab.[ch]
  LEX     scripts/kconfig/zconf.lex.c
  HOSTCC  scripts/kconfig/zconf.tab.o
  HOSTLD  scripts/kconfig/conf
#
# configuration written to .config
#
$ make -j$(nproc)
```

Binaries are at:

```shell
root@c6f070db0078:/workspace/u-boot-v2026.10# ls -l u-boot*
-rwxr-xr-x 1 root root 9045664 Nov 25 19:32 u-boot
-rwxr-xr-x 1 root root 1191192 Nov 25 19:32 u-boot-nodtb.bin
-rwxr-xr-x 1 root root 1191192 Nov 25 19:32 u-boot.bin
-rw-r--r-- 1 root root   15506 Nov 25 19:29 u-boot.cfg
-rw-r--r-- 1 root root    1134 Nov 25 19:29 u-boot.lds
-rw-r--r-- 1 root root 1383237 Nov 25 19:32 u-boot.map
-rwxr-xr-x 1 root root 3573644 Nov 25 19:32 u-boot.srec
-rw-r--r-- 1 root root  375113 Nov 25 19:32 u-boot.sym
```
