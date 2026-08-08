
all:
	@echo "targets: sync"

sync:
	rsync -avz sys/ root@192.168.42.1:/usr/src/sys/

sys.tgz:
	tar --no-xattrs -czf sys.tgz sys
