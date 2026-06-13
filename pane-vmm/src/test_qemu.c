#include "pane_vmm.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <unistd.h>
#include <fcntl.h>
#include <sys/stat.h>

int main() {
    printf("Setting up dummy disk image...\n");
    const char *img_path = "test_disk.img";
    int img_fd = open(img_path, O_CREAT | O_TRUNC | O_WRONLY, 0644);
    if (img_fd < 0) {
        perror("Failed to create dummy disk image");
        return 1;
    }
    if (ftruncate(img_fd, 1024 * 1024) < 0) { // 1 MB
        perror("Failed to truncate dummy disk image");
        close(img_fd);
        return 1;
    }
    close(img_fd);

    printf("Creating pane_vm...\n");
    pane_vm_t *vm = NULL;
    int ret = pane_vm_create(&vm);
    if (ret != 0) {
        fprintf(stderr, "pane_vm_create failed: %s\n", strerror(-ret));
        unlink(img_path);
        return 1;
    }

    printf("Setting up QEMU mode...\n");
    const char *qmp_path = "test_qmp.sock";
    ret = pane_vm_setup_qemu_mode(vm, img_path, qmp_path);
    if (ret != 0) {
        fprintf(stderr, "pane_vm_setup_qemu_mode failed: %s\n", strerror(-ret));
        pane_vm_destroy(vm);
        unlink(img_path);
        return 1;
    }

    printf("Querying status (should be running)...\n");
    char status[64];
    ret = pane_vm_qemu_query_status(vm, status, sizeof(status));
    if (ret != 0) {
        fprintf(stderr, "Query status failed: %s\n", strerror(-ret));
        pane_vm_destroy(vm);
        unlink(img_path);
        return 1;
    }
    printf("Status: %s\n", status);
    if (strcmp(status, "running") != 0) {
        fprintf(stderr, "Expected status 'running', got '%s'\n", status);
        pane_vm_destroy(vm);
        unlink(img_path);
        return 1;
    }

    printf("Suspending VM...\n");
    ret = pane_vm_qemu_suspend(vm);
    if (ret != 0) {
        fprintf(stderr, "Suspend failed: %s\n", strerror(-ret));
        pane_vm_destroy(vm);
        unlink(img_path);
        return 1;
    }

    printf("Querying status (should be paused)...\n");
    ret = pane_vm_qemu_query_status(vm, status, sizeof(status));
    if (ret != 0) {
        fprintf(stderr, "Query status failed: %s\n", strerror(-ret));
        pane_vm_destroy(vm);
        unlink(img_path);
        return 1;
    }
    printf("Status: %s\n", status);
    if (strcmp(status, "paused") != 0) {
        fprintf(stderr, "Expected status 'paused', got '%s'\n", status);
        pane_vm_destroy(vm);
        unlink(img_path);
        return 1;
    }

    printf("Resuming VM...\n");
    ret = pane_vm_qemu_resume(vm);
    if (ret != 0) {
        fprintf(stderr, "Resume failed: %s\n", strerror(-ret));
        pane_vm_destroy(vm);
        unlink(img_path);
        return 1;
    }

    printf("Querying status (should be running)...\n");
    ret = pane_vm_qemu_query_status(vm, status, sizeof(status));
    if (ret != 0) {
        fprintf(stderr, "Query status failed: %s\n", strerror(-ret));
        pane_vm_destroy(vm);
        unlink(img_path);
        return 1;
    }
    printf("Status: %s\n", status);
    if (strcmp(status, "running") != 0) {
        fprintf(stderr, "Expected status 'running', got '%s'\n", status);
        pane_vm_destroy(vm);
        unlink(img_path);
        return 1;
    }

    printf("Destroying VM...\n");
    pane_vm_destroy(vm);

    printf("Cleaning up disk image...\n");
    unlink(img_path);

    // Verify QMP socket was deleted
    struct stat st;
    if (stat(qmp_path, &st) == 0) {
        fprintf(stderr, "WARNING: QMP socket file %s was not deleted by destroy\n", qmp_path);
        unlink(qmp_path);
        return 1;
    }

    printf("All QEMU backend tests passed!\n");
    return 0;
}
