
EXAMPLES := hello print
STATIC := main stat types
DYNAMIC := uname
IMAGE := date uname

all: run-examples run-static run-dynamic run-image run-ls

run-examples:
	@for i in $(EXAMPLES); do \
	  echo "Examples/ $$i"; \
	  ./emulator examples/$$i; \
	done

run-static:
	@for i in $(STATIC); do \
	  echo "Static/ $$i"; \
	  ./emulator static/$$i; \
	done

run-dynamic:
	@for i in $(DYNAMIC); do \
	  echo "Dynamic/ $$i"; \
	  ./emulator -fsroot image dynamic/$$i; \
        done

run-image:
	@for i in $(IMAGE); do \
	  echo "Image/ $$i"; \
	  ./emulator -fsroot image image/bin/$$i; \
        done

run-ls:
	@echo "The ls -l emulator"
	@./emulator -fsroot image image/bin/ls -l emulator
