
all:
	@echo "targets: sync"

KERNEL_SRC := /usr/src

sync:
	scp sys/riscv/conf/GOEMU root@192.168.42.1:/usr/src/sys/riscv/conf/

sys.tgz:
	tar --no-xattrs -czf sys.tgz sys
