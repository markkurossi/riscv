# Debugging GoEMU, Linux Kernel, and Operating System Tools

## Linux Image

Converting from `rootfs.ext2` to `rootfs.cpio`:

``` shell
debugfs -R "rdump / ./rootfs_contents" rootfs.ext2
cd rootfs_contents
find . | cpio -o -H newc | gzip > ../rootfs.cpio.gz
```

## Device Tree

``` shell
$ dtc -I dtb -O dts -o source.dts goemu.dtb
```

## Building Linux Kernel

``` shell
cd ephemelier/riscv
make run
cd own/linux-7.0.11
```

### Configure

``` shell
make ARCH=riscv CROSS_COMPILE=riscv64-linux-gnu- defconfig
```

We could also tune the kernel parameters with:

``` shell
make ARCH=riscv CROSS_COMPILE=riscv64-linux-gnu- menuconfig
```

### Build

``` shell
make -j$(nproc) ARCH=riscv CROSS_COMPILE=riscv64-linux-gnu-
```

## Debugging ext2 images with Docker

``` shell
run-image:
	docker run --privileged \
	-v $(CURDIR):/workspace \
	-v $(CURDIR)/ubuntu-26.04-preinstalled-server-riscv64.img:/image.ext2 \
	-it riscv-toolchain
```

Inside Docker shell:

``` shell
$ kpartx -av /image.ext2
add map loop0p1 (253:0): 0 9209823 linear 7:0 227328
add map loop0p12 (253:1): 0 8192 linear 7:0 219136
add map loop0p15 (253:2): 0 217088 linear 7:0 2048
$ mkdir -p /mnt/ext2_inside
$ mount /dev/mapper/loop0p1 /mnt/ext2_inside/
$ file /mnt/ext2_inside/sbin/init
/mnt/ext2_inside/sbin/init: symbolic link to ../lib/systemd/systemd
$ file /mnt/ext2_inside/lib/systemd/systemd
/mnt/ext2_inside/lib/systemd/systemd: ELF 64-bit LSB pie executable, UCB RISC-V, RVC, double-float ABI, version 1 (SYSV), dynamically linked, interpreter /lib/ld-linux-riscv64-lp64d.so.1, BuildID[sha1]=16822c25a108c2b12ac2409154f34deaa82bfc1c, for GNU/Linux 4.15.0, stripped
```
