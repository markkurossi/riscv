# GoEMU OpenBSD Guest

OpenBSD boot, installation, and user-mode work with OpenBSD 7.9.

## Time with NMEA

Enable `ldattach`:

```shell
$ rcctl enable ldattach
$ rcctl set ldattach flags "nmea tty01"
```

Configure `/etc/ntpd.conf`. For some reason the constraints do not
work so comment them out:

```
# $OpenBSD: ntpd.conf,v 1.16 2019/11/06 19:04:12 deraadt Exp $
#
# See ntpd.conf(5) and /etc/examples/ntpd.conf

servers pool.ntp.org
server time.cloudflare.com
sensor *

#constraint from "9.9.9.9"              # quad9 v4 without DNS
#constraint from "2620:fe::fe"          # quad9 v6 without DNS
#constraints from "www.google.com"      # intentionally not 8.8.8.8
```

Checking that everything works:

```shell
$ ntpctl -s all
5/5 peers valid, 1/1 sensors valid, clock unsynced

peer
   wt tl st  next  poll          offset       delay      jitter
162.159.200.123 time.cloudflare.com
    1  9  3    8s   32s      4072.051ms    16.798ms    22.381ms
51.38.131.245 from pool pool.ntp.org
    1  9  2    6s   31s      4202.192ms    14.385ms    20.580ms
212.132.97.26 from pool pool.ntp.org
    1  9  2   16s   33s      3767.426ms    41.544ms    36.265ms
194.92.94.32 from pool pool.ntp.org
    1  9  2   17s   33s      3865.087ms    24.316ms    24.913ms
156.17.245.123 from pool pool.ntp.org
    1  9  2   13s   32s      3836.478ms    17.998ms    19.641ms

sensor
   wt gd st  next  poll          offset  correction
nmea0
    1  1  0    2s   15s      3208.592ms     0.000ms
$ sysctl hw.sensors
hw.sensors.nmea0.indicator0=On (Signal), OK
hw.sensors.nmea0.timedelta0=-7.010747 secs (GPS autonomous), OK, Fri Jul 31 12:11:39.949
hw.sensors.nmea0.angle0=0.0000 degrees (Latitude), OK
hw.sensors.nmea0.angle1=0.0000 degrees (Longitude), OK
hw.sensors.nmea0.velocity0=0.000 m/s (Ground speed), OK
```

## X11 and VirtIO GPU

### VirtIO GPU over MMIO

The virtio-gpu does not work over MMIO. The driver does not negotiate
the `VIRTIO_F_VERSION_1` with the upper 32-bit
`VIRTIO_MMIO_HOST_FEATURES_SEL`. The
[virtio-mmio.patch](virtio-mmio.patch) fixes that. After this patch,
the virtual console works and `startx` starts X11 desktop environment.

### Virtio Input not implemented

The OpenBSD does not have virtio-input driver. This means that virtual
console and X11 do not receive keyboard and mouse events.
