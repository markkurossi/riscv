./spike -m256 \
    --kernel cmd/goemu/linux-7.2.2/Image \
    --initrd tests/rootfs.cpio.gz \
    --bootargs "root=/dev/ram rw console=hvc0 earlycon=sbi" \
    cmd/goemu/opensbi/fw_jump.elf
