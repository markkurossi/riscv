# Network Performance Testing

Guest is server:

```shell
$ iperf3 -s
```

Host is client and runs normal client on `H->G` tests:

```shell
$ iperf3-darwin -c 192.168.42.1
```

and `-R` on `G->H` tests:

```shell
$ iperf3-darwin -c 192.168.42.1 -R
```

| Version      | H->G Sndr | H->G Rcvr | G->H Sndr | G->H Rcvr |
|:-------------|----------:|----------:|----------:|----------:|
| Baseline     |      58.5 |       116 |       103 |      44.2 |
| Async VIO    |      61.3 |       118 |       103 |      58.2 |
| Unsafe ld/sd |      68.3 |       116 |       103 |      62.8 |

# Disk Performance Testing

Sequential testing:

```shell
$ fio --name=seqwrite --filename=testfile --size=1G --rw=write \
    --bs=1M --iodepth=16 --direct=1 -ioengine=posixaio
```

| Version           | MiB/s |  iops | CPU [usr] | CPU [sys] |
|-------------------|------:|------:|----------:|----------:|
| Baseline          |  62.7 | 61.19 |      1.18 |      0.69 |
| Async VIO         |  63.1 | 61.94 |      1.75 |      0.12 |
| Optimized locking |  63.0 | 62.50 |      1.37 |      0.50 |

Random testing:

```shell
fio --name=randread \
    --filename=testfile \
    --size=1G \
    --rw=randread \
    --bs=4k \
    --iodepth=32 \
    --direct=1
```
