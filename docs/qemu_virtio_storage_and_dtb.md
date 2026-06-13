# QEMU VirtIO Storage on RISC-V

## Overview

QEMU separates storage into two layers:

1. **Storage backend** (`-drive`)
2. **Emulated device** (`-device`)

```text
Disk image file
      |
      v
   -drive
      |
      v
   Backend ID
      |
      v
   -device
      |
      v
 VirtIO MMIO device
      |
      v
 Linux (/dev/vda, /dev/vdb, ...)
```

---

## `-drive` Option

Creates a storage backend.

Example:

```bash
-drive file=rootfs.ext2,format=raw,id=disk0
```

### Common Parameters

| Parameter     | Description                         |
|---------------|-------------------------------------|
| `file=`       | Host image file                     |
| `format=`     | Image format (`raw`, `qcow2`, etc.) |
| `id=`         | Backend identifier                  |
| `readonly=on` | Read-only backend                   |
| `snapshot=on` | Discard changes when QEMU exits     |
| `cache=`      | Cache mode                          |
| `discard=`    | TRIM/discard support                |

Example:

```bash
-drive file=rootfs.ext2,format=raw,id=disk0
```

Creates:

```text
Backend:
    id=disk0
    file=rootfs.ext2
```

This backend is not visible to the guest until attached to a device.

---

## `-device` Option

Creates an emulated hardware device.

Example:

```bash
-device virtio-blk-device,drive=disk0
```

### Common Parameters

| Parameter    | Description                               |
|--------------|-------------------------------------------|
| Device type  | First argument (e.g. `virtio-blk-device`) |
| `drive=`     | Backend identifier to attach              |
| `id=`        | Internal QEMU device identifier           |
| `serial=`    | Disk serial number                        |
| `bootindex=` | Boot priority                             |

Example:

```bash
-device virtio-blk-device,drive=disk0
```

Creates a VirtIO block device and attaches backend `disk0`.

Linux sees:

```text
/dev/vda
```

---

## Modern Recommended Syntax

```bash
-drive if=none,file=rootfs.ext2,format=raw,id=disk0 \
-device virtio-blk-device,drive=disk0
```

Advantages:

- Clean separation of backend and frontend.
- Same backend can be attached to different controller types.
- Easier multi-device configuration.

---

## Multiple Disks

Example:

```bash
-drive if=none,file=rootfs.ext2,id=disk0,format=raw \
-device virtio-blk-device,drive=disk0 \

-drive if=none,file=data.ext2,id=disk1,format=raw \
-device virtio-blk-device,drive=disk1
```

Linux sees:

```text
/ dev/vda
/ dev/vdb
```

---

# Device Tree Generation

On the RISC-V `virt` machine, QEMU exposes VirtIO devices through VirtIO-MMIO.

Each VirtIO device receives:

- A dedicated MMIO region.
- A dedicated interrupt.
- A dedicated DTB node.

Example:

```dts
virtio@10008000 {
    compatible = "virtio,mmio";
    reg = <0x0 0x10008000 0x0 0x1000>;
    interrupts = <8>;
};
```

The DTB node identifies:

- MMIO base address
- MMIO size
- Interrupt number

The DTB does **not** identify the VirtIO device type.

Linux determines the type by reading:

```text
MMIO + 0x008 = DeviceID
```

Examples:

| DeviceID | Device       |
|----------|--------------|
| 1        | Network      |
| 2        | Block        |
| 3        | Console      |
| 4        | Entropy      |
| 9        | 9P Transport |

---

## Multiple VirtIO Devices

With two VirtIO block devices:

```dts
virtio@10008000 {
    compatible = "virtio,mmio";
    reg = <0x0 0x10008000 0x0 0x1000>;
    interrupts = <8>;
};

virtio@10009000 {
    compatible = "virtio,mmio";
    reg = <0x0 0x10009000 0x0 0x1000>;
    interrupts = <9>;
};
```

Linux probes:

```text
virtio@10008000 -> DeviceID=2 -> /dev/vda
virtio@10009000 -> DeviceID=2 -> /dev/vdb
```

---

# VirtIO MMIO Address Allocation

QEMU's RISC-V `virt` machine provides a fixed set of VirtIO-MMIO slots.

Typical layout:

```text
0x10001000
0x10002000
0x10003000
...
0x10008000
0x10009000
0x1000a000
...
```

Characteristics:

- Each slot occupies 0x1000 bytes.
- One VirtIO device per slot.
- Slots are assigned in device creation order.

Example:

```bash
-device virtio-blk-device,drive=disk0
-device virtio-blk-device,drive=disk1
-device virtio-net-device
```

Possible assignment:

```text
0x10008000 -> virtio-blk -> /dev/vda
0x10009000 -> virtio-blk -> /dev/vdb
0x1000a000 -> virtio-net
```

---

# Interrupt Allocation

Each VirtIO-MMIO device receives its own interrupt.

Example:

```text
virtio@10008000 -> IRQ 8
virtio@10009000 -> IRQ 9
virtio@1000a000 -> IRQ 10
```

The exact interrupt numbers depend on the machine configuration and
QEMU version, but:

- One interrupt is allocated per VirtIO device.
- Linux connects each interrupt to the corresponding VirtIO transport instance.
- Interrupts are routed through the PLIC.

---

# Linux Enumeration Flow

```text
DTB
 |
 +-- virtio@10008000
 |
 +-- virtio@10009000
 |
Linux virtio-mmio driver
 |
 +-- Read DeviceID
 |
 +-- DeviceID = 2
 |
 +-- virtio_blk
 |
 +-- /dev/vda
 |
 +-- /dev/vdb
```

The DTB only describes transport endpoints. Device discovery occurs by
reading the VirtIO MMIO registers.

----------------------------------------------------------------------

```
how could I make an empty device (file backed drive) to attach to the
goemu? I could create the file for a fixed size and then format and
mount that to the emulated ubuntu
```

Yes. That's exactly how you would create a second virtual disk for
Ubuntu running under GoEMU.

## 1. Create an empty image file

For example a 1 GB disk:

```bash
truncate -s 1G data.img
```

or:

```bash
dd if=/dev/zero of=data.img bs=1M count=1024
```

Verify:

```bash
ls -lh data.img
```

---

## 2. Attach it as another VirtIO block device

In GoEMU:

```go
blk0 := virtio.NewBlk("ubuntu.img", ...)
blk1 := virtio.NewBlk("data.img", ...)
```

and expose two MMIO devices:

```text
10008000 -> blk0 -> IRQ 13
10009000 -> blk1 -> IRQ 14
```

DTB:

```dts
virtio@10008000 {
    compatible = "virtio,mmio";
    reg = <0x0 0x10008000 0x0 0x1000>;
    interrupts = <13>;
};

virtio@10009000 {
    compatible = "virtio,mmio";
    reg = <0x0 0x10009000 0x0 0x1000>;
    interrupts = <14>;
};
```

Linux should then see:

```text
/dev/vda
/dev/vdb
```

---

## 3. Verify inside Ubuntu

```bash
lsblk
```

Example:

```text
NAME   MAJ:MIN RM SIZE RO TYPE MOUNTPOINT
vda    254:0    0  10G  0 disk
├─vda1
└─vda2

vdb    254:16   0   1G  0 disk
```

---

## 4. Create a partition table (optional)

Using GPT:

```bash
sudo fdisk /dev/vdb
```

Create:

```text
g    # GPT
n    # new partition
w    # write
```

or:

```bash
sudo parted /dev/vdb mklabel gpt
sudo parted /dev/vdb mkpart primary ext4 1MiB 100%
```

Result:

```text
/dev/vdb1
```

---

## 5. Create a filesystem

Ext4:

```bash
sudo mkfs.ext4 /dev/vdb1
```

or directly on the whole disk:

```bash
sudo mkfs.ext4 /dev/vdb
```

BusyBox:

```bash
mke2fs /dev/vdb1
```

---

## 6. Mount it

```bash
sudo mkdir /mnt/data
sudo mount /dev/vdb1 /mnt/data
```

Verify:

```bash
df -h
```

---

## 7. Persist via /etc/fstab

Find UUID:

```bash
sudo blkid /dev/vdb1
```

Example:

```text
UUID=12345678-1234-1234-1234-123456789abc
```

Add to:

```bash
sudo nano /etc/fstab
```

```fstab
UUID=12345678-1234-1234-1234-123456789abc /mnt/data ext4 defaults 0 2
```

---

## Alternative: create and format entirely on the host

You can also create the filesystem before booting Ubuntu:

```bash
truncate -s 1G data.img
mkfs.ext4 data.img
```

Linux can mount an ext4 filesystem directly from a block device image.

After attaching as `/dev/vdb`:

```bash
mount /dev/vdb /mnt/data
```

(no partition table needed).

This is actually the simplest approach for emulator testing because it
exercises only the VirtIO block layer and filesystem, not GPT
partition handling.

For a first test of multiple disks in GoEMU, I'd recommend:

```bash
truncate -s 1G data.img
mkfs.ext4 data.img
```

attach it as a second VirtIO block device, boot Ubuntu, and run:

```bash
mount /dev/vdb /mnt
touch /mnt/test
ls -l /mnt
```

That will immediately validate read/write support, multiple VirtIO
devices, interrupt routing, and persistence across reboots.

## Create a Shared Drive for macOS and Linux

```bash
$ lsblk
NAME    MAJ:MIN RM  SIZE RO TYPE MOUNTPOINTS
vda     254:0    0  4.5G  0 disk
├─vda1  254:1    0  4.4G  0 part /
├─vda12 254:12   0    4M  0 part
├─vda13 254:13   0    1M  0 part
├─vda14 254:14   0    4M  0 part
└─vda15 254:15   0  106M  0 part /boot/efi
vdb     254:16   0    1G  0 disk
└─vdb1  254:17   0 1024M  0 part
$ sudo fdisk /dev/vdb
g    # GPT
n    # new partition
w    # write
$ lsblk
NAME    MAJ:MIN RM  SIZE RO TYPE MOUNTPOINTS
vda     254:0    0  4.5G  0 disk
├─vda1  254:1    0  4.4G  0 part /
├─vda12 254:12   0    4M  0 part
├─vda13 254:13   0    1M  0 part
├─vda14 254:14   0    4M  0 part
└─vda15 254:15   0  106M  0 part /boot/efi
vdb     254:16   0    1G  0 disk
└─vdb1  254:17   0 1022M  0 part
sudo mkfs.exfat /dev/vda
```
