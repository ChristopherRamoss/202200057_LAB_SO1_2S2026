CLANG ?= clang
ARCH := $(shell uname -m | sed 's/x86_64/x86/' | sed 's/aarch64/arm64/')
MULTIARCH := $(shell gcc -dumpmachine)
BPF_CFLAGS := -g -O2 -target bpf -D__TARGET_ARCH_$(ARCH) -I/usr/include/$(MULTIARCH)

all: kill_probe.o

kill_probe.o: kill_probe.c
	$(CLANG) $(BPF_CFLAGS) -c kill_probe.c -o kill_probe.o

verify:
	sudo bpftool prog show | grep trace_kill || echo "sonda no adjuntada actualmente"

clean:
	rm -f kill_probe.o

.PHONY: all verify clean
