#include "pane_vmm.h"
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <fcntl.h>
#include <errno.h>
#include <string.h>

// Function to count number of open file descriptors for the current process
static long count_open_fds(void) {
    long maxfd = sysconf(_SC_OPEN_MAX);
    if (maxfd < 0) {
        maxfd = 1024; // fallback
    }
    long count = 0;
    for (long fd = 0; fd < maxfd; fd++) {
        if (fcntl(fd, F_GETFD) != -1 || errno != EBADF) {
            count++;
        }
    }
    return count;
}

int main(void) {
    long initial_fds = count_open_fds();
    printf("Initial open FDs: %ld\n", initial_fds);

    const int n_vms = 1000;
    pane_vm_t *vms[n_vms];

    for (int i = 0; i < n_vms; i++) {
        int ret = pane_vm_create(&vms[i]);
        if (ret != 0) {
            fprintf(stderr, "Failed to create VM %d: %s\n", i, strerror(-ret));
            // Clean up any successfully created VMs
            for (int j = 0; j < i; j++) {
                pane_vm_destroy(vms[j]);
            }
            return 1;
        }
    }

    printf("Created %d VMs\n", n_vms);

    long after_create_fds = count_open_fds();
    printf("After creating %d VMs, open FDs: %ld\n", n_vms, after_create_fds);

    // Destroy all VMs
    for (int i = 0; i < n_vms; i++) {
        pane_vm_destroy(vms[i]);
    }

    long after_destroy_fds = count_open_fds();
    printf("After destroying all VMs, open FDs: %ld\n", after_destroy_fds);

    // Check for fd leak: the number of FDs should be the same as initial (within reason)
    // Note: there might be some internal fds opened by the kernel, but we expect no leak from our code.
    // We'll just check that we didn't leak a large number.
    if (after_destroy_fds > initial_fds + 10) { // allow a small slack for things like stderr, etc.
        fprintf(stderr, "Possible FD leak: initial %ld, after destroy %ld\n", initial_fds, after_destroy_fds);
        return 1;
    }

    printf("Test passed: no significant FD leak detected.\n");
    return 0;
}