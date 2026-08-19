# NetBSD

``` shell
netbsd$ ./rtime/time cc -o hello hello.c
       3.140885 real         2.285269 user         0.724326 sys
               27156 maximum resident set size
               46676 average shared memory size
                4596 average unshared data size
                  64 average unshared stack size
                4429 page reclaims
                   0 page faults
                   0 swaps
                   0 block input operations
                  20 block output operations
                   0 messages sent
                   0 messages received
                   0 signals received
                  26 voluntary context switches
                  62 involuntary context switches
           302057245 instructions retired
           302058201 cycles elapsed
               96.17 MIPS
```

# FreeBSD

``` shell
mtr@freebsd:~ $ ./riscv/time cc -o hello hello.c
       2.974676 real         2.117478 user         1.204365 sys
               65444 maximum resident set size
              239692 average shared memory size
                3008 average unshared data size
               53504 average unshared stack size
                5542 page reclaims
                   0 page faults
                   0 swaps
                   0 block input operations
                   6 block output operations
                   0 messages sent
                   0 messages received
                   0 signals received
                  68 voluntary context switches
                 202 involuntary context switches
           332780553 instructions retired
           332784302 cycles elapsed
              111.87 MIPS
```
