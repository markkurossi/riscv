# Ubuntu

Check how to disable systemd. It is using +35% CPU if host network is
down.

## Grow Image

Host:

``` shell
# Add 5 GB to the image
$ truncate -s +5G rootfs.img
```

Guest:

``` shell
$ df -h
$ lsblk -f
$ sudo growpart /dev/vda 1
$ sudo resize2fs /dev/vda1
```
