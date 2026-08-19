all: time

time: time.c hwcap.c
	$(CC) -Wall -o $@ $^

TARGET := mtr@192.168.42.1:riscv/
TARGET_LINUX := mtr@192.168.42.2:riscv/
TARGET_NETBSD := mtr@192.168.42.3:rtime/


push:
	rsync -avhe ssh --delete --exclude-from rsync.exclude . $(TARGET)

push-linux:
	rsync -avhe ssh --delete --exclude-from rsync.exclude . $(TARGET_LINUX)

push-netbsd:
	scp *.c Makefile $(TARGET_NETBSD)
