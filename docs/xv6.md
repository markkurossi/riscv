# Xv6: a simple, Unix-like teaching operating system

The GoEMU boots Xv6 with two config settings:
 - `compat_mode`: Xv6 uses MMIOv2 but it does not negotiate
   `VIRTIO_F_VERSION_1`
 - `vendor_id`: Xv6 requires QOEMU MMIO vendor ID `0x554d4551`

These are configured in the `xv6.goemu` drive block:

```json
  "drives": [
    {
      "id":          "hd0",
      "format":      "raw",
      "compat_mode": true,
      "vendor_id":   "0x554d4551",
      "file":        "xv6/fs.img"
    }
  ],
```

- [Homepage](https://pdos.csail.mit.edu/6.1810/2026/xv6.html)
- [Source code](https://github.com/mit-pdos/xv6-riscv)
- Book:
  - [Source](https://github.com/mit-pdos/xv6-riscv-book)
  - [Online](https://mit-pdos.github.io/xv6-riscv-book/)
