
all:
	@echo "targets: sync"

sync:
	rsync -avz sys/ root@192.168.42.1:/usr/src/sys/
	scp fbinfo.c ../../cmd/rtime/goemu.h mtr@192.168.42.1:

sys.tgz:
	tar --no-xattrs -czf sys.tgz sys
