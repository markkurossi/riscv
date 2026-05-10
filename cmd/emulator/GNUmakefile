
EXAMPLES := hello print
STATIC := main stat types
DYNAMIC := uname
IMAGE := date uname

all: run-examples run-static run-dynamic run-image run-ls

run-examples:
	@for i in $(EXAMPLES); do echo "*** $$i"; ./emulator examples/$$i; done

run-static:
	@for i in $(STATIC); do echo "*** $$i"; ./emulator static/$$i; done

run-dynamic:
	@for i in $(DYNAMIC); do \
	  echo "*** $$i"; \
	  ./emulator -fsroot image dynamic/$$i; \
        done

run-image:
	@for i in $(IMAGE); do \
	  echo "*** $$i"; \
	  ./emulator -fsroot image image/bin/$$i; \
        done

run-ls:
	@echo "*** ls -l emulator"
	@./emulator -fsroot image image/bin/ls -l emulator
