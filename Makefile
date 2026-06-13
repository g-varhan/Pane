.PHONY: test bench

test:
	$(MAKE) -C pane-vmm test

bench:
	$(MAKE) -C benchmarks bench

# Default target
all: test