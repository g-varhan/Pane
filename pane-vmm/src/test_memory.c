#include "pane_vmm.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <unistd.h>
#include <fcntl.h>
#include <sys/mman.h>
#include <sys/ioctl.h>
#include <linux/kvm.h>

int main(void) {
    pane_vm_t *vm = NULL;
    int ret;

    // Test 1: Create VM
    ret = pane_vm_create(&vm);
    if (ret != 0) {
        fprintf(stderr, "Failed to create VM: %s\n", strerror(-ret));
        return 1;
    }
    printf("VM created successfully\n");

    // Test 2: Get fds
    int kvm_fd = pane_vm_get_kvm_fd(vm);
    int vm_fd = pane_vm_get_vm_fd(vm);
    if (kvm_fd < 0 || vm_fd < 0) {
        fprintf(stderr, "Invalid fds: kvm_fd=%d, vm_fd=%d\n", kvm_fd, vm_fd);
        pane_vm_destroy(vm);
        return 1;
    }
    printf("KVM fd: %d, VM fd: %d\n", kvm_fd, vm_fd);

    // Test 3: Set a valid memory region
    // We'll use slot 0, guest phys addr 0x10000000, size 64MB, userspace addr 0x20000000
    // Note: We are not actually backing this memory, so the ioctl might fail if the address is not accessible.
    // But we are only testing the function call and error handling.
    // We'll use MAP_ANONYMOUS | MAP_PRIVATE to allocate some userspace memory for the test.
    void *userspace = mmap(NULL, 64*1024*1024, PROT_READ | PROT_WRITE, MAP_ANONYMOUS | MAP_PRIVATE, -1, 0);
    if (userspace == MAP_FAILED) {
        perror("mmap");
        pane_vm_destroy(vm);
        return 1;
    }
    ret = pane_vm_set_user_memory_region(vm, 0, 0x10000000, 64*1024*1024, (uint64_t)userspace, 0);
    if (ret != 0) {
        fprintf(stderr, "Failed to set memory region: %s\n", strerror(-ret));
        munmap(userspace, 64*1024*1024);
        pane_vm_destroy(vm);
        return 1;
    }
    printf("Memory region set successfully\n");

    // Test 4: Set another slot
    ret = pane_vm_set_user_memory_region(vm, 1, 0x20000000, 32*1024*1024, (uint64_t)userspace + 64*1024*1024, 0);
    if (ret != 0) {
        fprintf(stderr, "Failed to set memory region slot 1: %s\n", strerror(-ret));
        munmap(userspace, 64*1024*1024);
        pane_vm_destroy(vm);
        return 1;
    }
    printf("Second memory region set successfully\n");

    // Test 5: Test error - slot out of bounds
    ret = pane_vm_set_user_memory_region(vm, PANE_VMM_MAX_MEM_SLOTS, 0, 0, 0, 0);
    if (ret != -EINVAL) {
        fprintf(stderr, "Expected -EINVAL for out of bounds slot, got %d\n", ret);
        munmap(userspace, 64*1024*1024);
        pane_vm_destroy(vm);
        return 1;
    }
    printf("Correctly rejected out of bounds slot\n");

    // Test 6: Test error - null vm
    ret = pane_vm_set_user_memory_region(NULL, 0, 0, 0, 0, 0);
    if (ret != -EINVAL) {
        fprintf(stderr, "Expected -EINVAL for null vm, got %d\n", ret);
        munmap(userspace, 64*1024*1024);
        pane_vm_destroy(vm);
        return 1;
    }
    printf("Correctly rejected null vm\n");

    // Cleanup
    munmap(userspace, 64*1024*1024);
    pane_vm_destroy(vm);
    printf("All tests passed!\n");
    return 0;
}
