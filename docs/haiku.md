# Haiku

## Compilation

```shell
$ cd haiku/haiku/generated.riscv64
$ ../../buildtools/jam/jam0 -j2 -q @minimum-mmc
$ ls -la haiku-mmc.image
-rw-rw-r-- 1 mtr mtr 348129280 Sep  3 10:23 haiku-mmc.image
$ ls -la objects/haiku/riscv64/release/system/kernel/kernel_riscv64
-rwxrwxr-x 1 mtr mtr 2193833 Sep  3 10:09 objects/haiku/riscv64/release/system/kernel/kernel_riscv64
```
