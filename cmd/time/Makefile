all: time

time: time.c
	$(CC) -Wall -o $@ $<

TARGET := mtr@192.168.42.1:riscv/


push:
	rsync -avhe ssh --delete --exclude-from rsync.exclude . $(TARGET)
