
EXAMPLES := hello print
STATIC := main stat types
DYNAMIC := uname
IMAGE := date uname

all: run-examples run-static run-dynamic run-image run-ls

run-examples:
	@for i in $(EXAMPLES); do \
	  echo "Examples/ $$i"; \
	  ./goemu examples/$$i; \
	done

run-static:
	@for i in $(STATIC); do \
	  echo "Static/ $$i"; \
	  ./goemu static/$$i; \
	done

run-dynamic:
	@for i in $(DYNAMIC); do \
	  echo "Dynamic/ $$i"; \
	  ./goemu -fsroot image dynamic/$$i; \
        done

run-image:
	@for i in $(IMAGE); do \
	  echo "Image/ $$i"; \
	  ./goemu -fsroot image image/bin/$$i; \
        done

run-ls:
	@echo "The ls -l bin/busybox"
	@./goemu -fsroot image image/bin/ls -l bin/busybox

netbsd_kernel_dir := ../../tests/netbsd/src/sys/arch/riscv/compile/obj/GENERIC64

get-netbsd:
	ls -l $(netbsd_kernel_dir)/netbsd
	cp  $(netbsd_kernel_dir)/netbsd netbsd/
	cp  $(netbsd_kernel_dir)/netbsd.map netbsd/
