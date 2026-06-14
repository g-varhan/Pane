CFLAGS ?= -Wall -Wextra -Werror -fsanitize=address,undefined -O2
INCLUDES = -Iinclude
LIBS = -luring

SRCS = src/kvm.o src/virtio.o src/uring.o src/backends/firecracker.o src/backends/qemu.o

TEST_BASES = test_create_destroy test_memory test_firecracker test_qemu test_uring_bench
TESTS = $(TEST_BASES)
ALL_TARGETS = $(TESTS) test_boot_serial

all: $(ALL_TARGETS)

test_create_destroy: src/test_create_destroy.o $(SRCS)
	$(CC) $(CFLAGS) $(INCLUDES) -o $@ $^ $(LIBS)

test_memory: src/test_memory.o $(SRCS)
	$(CC) $(CFLAGS) $(INCLUDES) -o $@ $^ $(LIBS)

test_boot_serial: src/test_boot_serial.o $(SRCS)
	$(CC) $(CFLAGS) $(INCLUDES) -o $@ $^ $(LIBS)

test_firecracker: src/test_firecracker.o $(SRCS)
	$(CC) $(CFLAGS) $(INCLUDES) -o $@ $^ $(LIBS)

test_qemu: src/test_qemu.o $(SRCS)
	$(CC) $(CFLAGS) $(INCLUDES) -o $@ $^ $(LIBS)

test_uring_bench: src/test_uring_bench.o $(SRCS)
	$(CC) $(CFLAGS) $(INCLUDES) -o $@ $^ $(LIBS)

%.o: %.c
	$(CC) $(CFLAGS) $(INCLUDES) -c -o $@ $<

test: $(TESTS)
	@for t in $(TESTS); do \
		echo "Running test: $$t"; \
		./$$t || exit 1; \
	done

clean:
	rm -f src/*.o src/backends/*.o $(ALL_TARGETS) *.img *.sock

.PHONY: all test clean
