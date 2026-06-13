#ifndef PANE_VMM_INTERNAL_H
#define PANE_VMM_INTERNAL_H

#include "pane_vmm.h"
#include <linux/kvm.h>
#include <sys/types.h>

#define PANE_VMM_MAX_MEM_SLOTS 256
#define PANE_VMM_MAX_VCPUS 32

struct pane_vcpu {
    int fd;
    struct kvm_run *run;
    uint32_t id;
};

struct pane_vm {
    int kvm_fd;
    int vm_fd;
    struct kvm_userspace_memory_region mem_slots[PANE_VMM_MAX_MEM_SLOTS];
    struct pane_vcpu vcpus[PANE_VMM_MAX_VCPUS];
    int vcpu_count;
    int vcpu_mmap_size;
    void *virtio_dev; // Opaque pointer to virtio device state

    // QEMU Backend fields
    pane_backend_t backend;
    pid_t qemu_pid;
    int qmp_fd;
    char *qmp_path;
};

#endif // PANE_VMM_INTERNAL_H
