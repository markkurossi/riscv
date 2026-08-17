# FreeBSD

## ntp

Edit `/etc/devfs.conf`:

``` shell
link ttyu1 gps1
```

``` shell
$ service devfs restart
```

edit `/etc/ntp.conf`:

``` shell
# GPS Serial Data (NMEA) on /dev/gps1
server 127.127.20.1 prefer minpoll 4 maxpoll 4
fudge 127.127.20.1 refid GPS
```

restart ntpd:

``` shell
$ sysrc ntpd_enable="YES"
$ service ntpd restart
$ ntpq -pn
```

## Kernel

Build:

```shell
$ cd /usr/src
$ make -j 4 kernel KERNCONF=GOEMU MODULES_OVERRIDE=""
$ reboot
```

# X11

``` shell
$ pkg install xorg-server
$ pkg install xf86-input-libinput
$ pkg install xf86-video-scfb
$ pkg install xinit xauth xset xterm xeyes xload xclock twm
$ pkg install font-misc-misc fontconfig xorg-fonts
```

# Virtio / FreeBSD / X11 TODO

## High Priority (Correctness)

- [ ] Fix ABS range mismatch
  - [ ] Update ABS_X to 0–65535
  - [ ] Update ABS_Y to 0–65535
  - [ ] Verify correct scaling in X/libinput

## Low Priority (Performance / Polish)

- [ ] Implement event batching
  - [ ] Batch multiple events per interrupt
  - [ ] Reduce VM exits
  - [ ] Check if we can detect framebuffer sync from FreeBSD kernel:
    - [ ] struct drm_mode_config_helper_funcs: atomic_commit{,_tail}

## Bonus (Nice-to-Have)

- [ ] Improve display reporting
  - [ ] Verify resolution handling
  - [ ] Investigate EDID support

## XDM

``` shell
sysrc xdm_enable="YES"
```

or

``` shell
$ vi /etc/ttys
ttyv8   "/usr/local/bin/xdm -nodaemon"   xterm   off  secure
=>
ttyv8   "/usr/local/bin/xdm -nodaemon"   xterm   on   secure
```

and create `.xsession`:

``` shell
echo "exec twm" > ~/.xsession # Or replace 'twm' with your chosen window manager
chmod +x ~/.xsession
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
