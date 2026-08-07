# FreeBSD

## Kernel

Build:

```shell
$ cd /usr/src
$ make kernel KERNCONF=GOEMU MODULES_OVERRIDE=""
$ make installkernel KERNCONF=GOEMU MODULES_OVERRIDE="" # XXX is this needed
$ reboot
```

## ports

``` shell
$ tar -xf ports-main.tar.gz -C /usr
$ mv /usr/ports-main /usr/ports
$ cd /usr/ports/x11/xorg
$ make install clean
```

## Compiling git

### Compile curl

``` shell
$ ./configure --with-openssl --without-libpsl
```

### Compile git

The `-O0` is needed to prevent clang compiler error:

``` shell
CC sha1dc/sha1.o

error: ran out of registers during register allocation

1 error generated.
```

``` shell
$ ./configure --with-curl=/usr/local
$ gmake NO_RUST=1 CFLAGS="-O0 -I/usr/local/include" LDFLAGS="-L/usr/local/lib -liconv"
```

## Expanding image

Host:

``` shell
$ truncate -s +1G FreeBSD-15.1-RELEASE-riscv-riscv64-GENERICSD.img
```

Guest:

``` shell
# gpart recover vtbd0
# gpart resize -i 4 vtbd0
# growfs /
```
