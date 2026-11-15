all: rtime

rtime: rtime.c hwcap.c
	$(CC) -Wall -o $@ $^

TARGET := mtr@192.168.42.1:rtime/
TARGET_LINUX := mtr@192.168.42.2:rtime/
TARGET_DEBIAN := mtr@192.168.42.5:rtime/
TARGET_NETBSD := mtr@192.168.42.3:rtime/

clean:
	rm -f rtime

push:
	rsync -avhe ssh --delete --exclude-from rsync.exclude . $(TARGET)

push-linux:
	rsync -avhe ssh --delete --exclude-from rsync.exclude . $(TARGET_LINUX)

push-debian:
	rsync -avhe ssh --delete --exclude-from rsync.exclude . $(TARGET_DEBIAN)

push-netbsd:
	scp *.c *.h Makefile $(TARGET_NETBSD)
