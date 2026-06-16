# Docker + GNUmakefile to build RISC-V binaries

All commands run in the docker environment:

``` shell
$ make run
```

## Das U-Boot

Download [U-Boot](https://u-boot.org/)

``` shell
$ cd u-boot-2026.04
$ export ARCH=riscv
$ export CROSS_COMPILE=riscv64-linux-gnu-
$ make qemu-riscv64_smode_defconfig
$ make -j$(nproc)
```

## NetBSD

Download
[NetBSD-current](https://ftp.netbsd.org/pub/NetBSD/NetBSD-current/tar_files/src.tar.gz)
and untar it in the [netbsd](netbsd/) directory.

``` shell
$ cd netbsd
$ tar xvzf ../src.tar.gz
```

Build NetBSD kernel:

``` shell
$ cd netbsd/src
$ ./build.sh -m riscv -a riscv64 -U -u kernel=GENERIC64
```

``` shell
 riscv64-unknown-elf-readelf -l sys/arch/riscv/compile/obj/GENERIC64/netbsd

Elf file type is EXEC (Executable file)
Entry point 0xffffffc000000000
There are 5 program headers, starting at offset 64

Program Headers:
  Type           Offset             VirtAddr           PhysAddr
                 FileSiz            MemSiz              Flags  Align
  RISCV_ATTRIBUT 0x00000000007ee274 0x0000000000000000 0x0000000000000000
                 0x0000000000000078 0x0000000000000000  R      0x1
  LOAD           0x0000000000001000 0xffffffc000000000 0x0000000080200000
                 0x000000000047fd04 0x000000000047fd04  R E    0x1000
  LOAD           0x0000000000481000 0xffffffc000600000 0x0000000080800000
                 0x0000000000203100 0x0000000000203100  R      0x1000
  LOAD           0x0000000000685000 0xffffffc000a00000 0x0000000080c00000
                 0x000000000016925c 0x0000000000200000  RW     0x1000
  GNU_STACK      0x0000000000000000 0x0000000000000000 0x0000000000000000
                 0x0000000000000000 0x0000000000000000  RW     0x10

 Section to Segment mapping:
  Segment Sections...
   00     .riscv.attributes
   01     .text
   02     .rodata .eh_frame link_set_evcnts link_set_sysctl_funcs link_set_modules link_set_fdt_platforms link_set_ieee80211_funcs link_set_domains link_set_sysdflt_device_calls link_set_dkwedge_methods link_set_of_device_calls link_set_fdt_consoles link_set_prop_linkpools
   03     .data .data.cacheline_aligned .data.read_mostly .sdata .bss
   04
```

### Build the whole NetBSD system:

``` shell
$ ./build.sh -m riscv -a riscv64 -U -u release
```

## Booting FreeBSD on QEMU

``` shell
$ qemu-system-riscv64 -machine virt -m 2048 -nographic \
    -bios fw_jump.bin \
    -kernel u-boot.bin \
    -drive file=FreeBSD-15.1-RC3-riscv-riscv64-GENERICSD.img,format=raw,id=hd0,if=none \
    -device virtio-blk-device,drive=hd0
```
