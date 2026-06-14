#ifndef PANE_VMM_INTERNAL_H
#define PANE_VMM_INTERNAL_H

#include "pane_vmm.h"
#include <linux/kvm.h>
#include <sys/types.h>
#include <liburing.h>

#define PANE_VMM_MAX_MEM_SLOTS 256
#define PANE_VMM_MAX_VCPUS 32

struct pane_vcpu {
    int fd;
    struct kvm_run *run;
    uint32_t id;
};

#define PANE_VMM_MAX_VIRTIO_DEVS 8

struct virtio_mmio_dev {
    uint64_t base_addr;
    uint64_t size;
    int irq;
    int (*handle_mmio)(struct virtio_mmio_dev *dev, uint64_t phys_addr, uint8_t *data, uint32_t len, uint8_t is_write);
    void (*free_dev)(struct virtio_mmio_dev *dev);
};

struct pane_vm {
    int kvm_fd;
    int vm_fd;
    struct kvm_userspace_memory_region mem_slots[PANE_VMM_MAX_MEM_SLOTS];
    struct pane_vcpu vcpus[PANE_VMM_MAX_VCPUS];
    int vcpu_count;
    int vcpu_mmap_size;
    struct virtio_mmio_dev *virtio_devs[PANE_VMM_MAX_VIRTIO_DEVS];
    int virtio_dev_count;

    // io_uring Engine
    struct io_uring *ring;

    // QEMU Backend fields
    pane_backend_t backend;
    pid_t qemu_pid;
    int qmp_fd;
    char *qmp_path;
};

int pane_vm_register_virtio_dev(pane_vm_t *vm, struct virtio_mmio_dev *dev);
void *pane_vm_gpa_to_hva(pane_vm_t *vm, uint64_t gpa);

#endif // PANE_VMM_INTERNAL_H
