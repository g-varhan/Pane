CFLAGS ?= -Wall -Wextra -Werror -fsanitize=address,undefined -O2
INCLUDES = -Iinclude

SRCS = src/kvm.o src/virtio.o src/backends/firecracker.o

TEST_BASES = test_create_destroy test_memory test_firecracker
TESTS = $(TEST_BASES)
ALL_TARGETS = $(TESTS) test_boot_serial

all: $(ALL_TARGETS)

test_create_destroy: src/test_create_destroy.o $(SRCS)
	$(CC) $(CFLAGS) $(INCLUDES) -o $@ $^

test_memory: src/test_memory.o $(SRCS)
	$(CC) $(CFLAGS) $(INCLUDES) -o $@ $^

test_boot_serial: src/test_boot_serial.o $(SRCS)
	$(CC) $(CFLAGS) $(INCLUDES) -o $@ $^

test_firecracker: src/test_firecracker.o $(SRCS)
	$(CC) $(CFLAGS) $(INCLUDES) -o $@ $^

%.o: %.c
	$(CC) $(CFLAGS) $(INCLUDES) -c -o $@ $<

test: $(TESTS)
	@for t in $(TESTS); do \
		echo "Running test: $$t"; \
		./$$t || exit 1; \
	done

clean:
	rm -f src/*.o src/backends/*.o $(ALL_TARGETS)

.PHONY: all test clean
