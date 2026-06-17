# NetBSD

``` shell
netbsd# ./time cc -o hello hello.c
       32.119737 user         0.693985 sys
     27400 maximum resident set size
     46676 average shared memory size
      4596 average unshared data size
        60 average unshared stack size
      4476 page reclaims
         0 page faults
         0 swaps
         0 block input operations
        22 block output operations
         0 messages sent
         0 messages received
         0 signals received
        26 voluntary context switches
       411 involuntary context switches
```

# FreeBSD

``` shell
root@freebsd:~ # ./time cc -o hello hello.c
        2.102534 user         1.259945 sys
     64600 maximum resident set size
    241808 average shared memory size
      3032 average unshared data size
     54016 average unshared stack size
      5539 page reclaims
         0 page faults
         0 swaps
         1 block input operations
         6 block output operations
         0 messages sent
         0 messages received
         0 signals received
        76 voluntary context switches
       221 involuntary context switches
         0 instructions retired
         0 cycles elapsed
```
