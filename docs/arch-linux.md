# Boot Issues

If the boot fails because /dev/ttys0 times out, edit the kernel boot
parameters to disable the tty timeout. Pause GRUB boot by pressing
`e`. Edit boot command line by appending the following parameters to
the kernel line:

```
console=ttyS0,115200 systemd.mask=dev-ttyS0.device systemd.wants=console-getty.service
```

Boot by pressing `C-x`.
