#include "pane_vmm.h"
#include <linux/kvm.h>
#include <sys/ioctl.h>
#include <unistd.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <errno.h>

// Declarations of internal helper functions exposed by kvm.c
void *pane_vm_get_virtio_dev(pane_vm_t *vm);
void pane_vm_set_virtio_dev(pane_vm_t *vm, void *dev);
void *pane_vm_gpa_to_hva(pane_vm_t *vm, uint64_t gpa);

#define VIRTIO_MMIO_MAGIC_VALUE         0x000
#define VIRTIO_MMIO_VERSION             0x004
#define VIRTIO_MMIO_DEVICE_ID           0x008
#define VIRTIO_MMIO_VENDOR_ID           0x00c
#define VIRTIO_MMIO_DEVICE_FEATURES     0x010
#define VIRTIO_MMIO_DEVICE_FEATURES_SEL 0x014
#define VIRTIO_MMIO_DRIVER_FEATURES     0x020
#define VIRTIO_MMIO_DRIVER_FEATURES_SEL 0x024
#define VIRTIO_MMIO_QUEUE_SEL           0x030
#define VIRTIO_MMIO_QUEUE_NUM_MAX       0x034
#define VIRTIO_MMIO_QUEUE_NUM           0x038
#define VIRTIO_MMIO_QUEUE_READY         0x044
#define VIRTIO_MMIO_QUEUE_NOTIFY        0x050
#define VIRTIO_MMIO_INTERRUPT_STATUS    0x060
#define VIRTIO_MMIO_INTERRUPT_ACK       0x064
#define VIRTIO_MMIO_STATUS              0x070
#define VIRTIO_MMIO_QUEUE_DESC_LOW      0x080
#define VIRTIO_MMIO_QUEUE_DESC_HIGH     0x084
#define VIRTIO_MMIO_QUEUE_AVAIL_LOW     0x090
#define VIRTIO_MMIO_QUEUE_AVAIL_HIGH    0x094
#define VIRTIO_MMIO_QUEUE_USED_LOW      0x0a0
#define VIRTIO_MMIO_QUEUE_USED_HIGH     0x0a4

// Virtio descriptor flags
#define VIRTQ_DESC_F_NEXT  1
#define VIRTQ_DESC_F_WRITE 2

struct virtq_desc {
    uint64_t addr;
    uint32_t len;
    uint16_t flags;
    uint16_t next;
};

struct virtq_avail {
    uint16_t flags;
    uint16_t idx;
    uint16_t ring[];
};

struct virtq_used_elem {
    uint32_t id;
    uint32_t len;
};

struct virtq_used {
    uint16_t flags;
    uint16_t idx;
    struct virtq_used_elem ring[];
};

struct virtio_queue {
    uint32_t num;
    uint32_t ready;
    uint64_t desc_gpa;
    uint64_t avail_gpa;
    uint64_t used_gpa;
    uint16_t last_avail_idx;
};

struct virtio_mmio_console {
    pane_vm_t *vm;
    uint64_t base_addr;
    uint64_t size;
    int irq;

    uint32_t device_features_sel;
    uint32_t driver_features_sel;
    uint32_t driver_features;
    uint32_t queue_sel;
    uint32_t interrupt_status;
    uint32_t status;

    struct virtio_queue queues[2]; // 0: RX, 1: TX
};

int pane_virtio_mmio_init(pane_vm_t *vm, uint64_t base_addr, uint64_t size, int irq) {
    struct virtio_mmio_console *dev = calloc(1, sizeof(struct virtio_mmio_console));
    if (!dev) {
        return -ENOMEM;
    }
    dev->vm = vm;
    dev->base_addr = base_addr;
    dev->size = size;
    dev->irq = irq;
    dev->status = 0;

    // Set default queue size max
    dev->queues[0].num = 256;
    dev->queues[1].num = 256;

    pane_vm_set_virtio_dev(vm, dev);
    return 0;
}

static void virtio_mmio_inject_irq(struct virtio_mmio_console *dev) {
    struct kvm_irq_level level = {
        .irq = dev->irq,
        .level = 1,
    };
    ioctl(pane_vm_get_vm_fd(dev->vm), KVM_IRQ_LINE, &level);
    level.level = 0;
    ioctl(pane_vm_get_vm_fd(dev->vm), KVM_IRQ_LINE, &level);
}

static void virtio_console_process_tx(struct virtio_mmio_console *dev) {
    struct virtio_queue *q = &dev->queues[1];
    if (!q->ready || q->desc_gpa == 0) return;

    struct virtq_desc *desc_table = pane_vm_gpa_to_hva(dev->vm, q->desc_gpa);
    struct virtq_avail *avail_ring = pane_vm_gpa_to_hva(dev->vm, q->avail_gpa);
    struct virtq_used *used_ring = pane_vm_gpa_to_hva(dev->vm, q->used_gpa);

    if (!desc_table || !avail_ring || !used_ring) {
        return;
    }

    while (q->last_avail_idx != avail_ring->idx) {
        uint16_t head = avail_ring->ring[q->last_avail_idx % q->num];
        uint16_t curr = head;

        // Process descriptor chain
        while (1) {
            void *buf = pane_vm_gpa_to_hva(dev->vm, desc_table[curr].addr);
            if (buf && desc_table[curr].len > 0) {
                // Write guest data to host stdout
                ssize_t written = write(STDOUT_FILENO, buf, desc_table[curr].len);
                (void)written;
            }

            if (!(desc_table[curr].flags & VIRTQ_DESC_F_NEXT)) {
                break;
            }
            curr = desc_table[curr].next;
        }

        // Put head descriptor in the used ring
        uint32_t used_idx = used_ring->idx % q->num;
        used_ring->ring[used_idx].id = head;
        used_ring->ring[used_idx].len = 0;
        used_ring->idx++;

        q->last_avail_idx++;
    }

    // Inject interrupt to notify guest
    dev->interrupt_status |= 0x1;
    virtio_mmio_inject_irq(dev);
}

int pane_handle_mmio(pane_vm_t *vm, uint64_t phys_addr, uint8_t *data, uint32_t len, uint8_t is_write) {
    struct virtio_mmio_console *dev = pane_vm_get_virtio_dev(vm);
    if (!dev) {
        return -EINVAL;
    }

    // Check if the address is in our MMIO range
    if (phys_addr < dev->base_addr || phys_addr >= dev->base_addr + dev->size) {
        return -ENOENT;
    }

    uint64_t offset = phys_addr - dev->base_addr;

    if (is_write) {
        if (len != 4) return -EINVAL;
        uint32_t val;
        memcpy(&val, data, 4);

        switch (offset) {
            case VIRTIO_MMIO_DEVICE_FEATURES_SEL:
                dev->device_features_sel = val;
                break;
            case VIRTIO_MMIO_DRIVER_FEATURES_SEL:
                dev->driver_features_sel = val;
                break;
            case VIRTIO_MMIO_DRIVER_FEATURES:
                if (dev->driver_features_sel == 1) {
                    dev->driver_features = val;
                }
                break;
            case VIRTIO_MMIO_QUEUE_SEL:
                if (val < 2) {
                    dev->queue_sel = val;
                }
                break;
            case VIRTIO_MMIO_QUEUE_NUM: {
                uint32_t qsel = dev->queue_sel;
                if (qsel < 2) {
                    dev->queues[qsel].num = val;
                }
                break;
            }
            case VIRTIO_MMIO_QUEUE_READY: {
                uint32_t qsel = dev->queue_sel;
                if (qsel < 2) {
                    dev->queues[qsel].ready = val;
                }
                break;
            }
            case VIRTIO_MMIO_QUEUE_NOTIFY:
                if (val == 1) { // TX queue notify
                    virtio_console_process_tx(dev);
                }
                break;
            case VIRTIO_MMIO_INTERRUPT_ACK:
                dev->interrupt_status &= ~val;
                break;
            case VIRTIO_MMIO_STATUS:
                dev->status = val;
                if (val == 0) {
                    memset(dev->queues, 0, sizeof(dev->queues));
                    dev->queues[0].num = 256;
                    dev->queues[1].num = 256;
                    dev->status = 0;
                    dev->interrupt_status = 0;
                }
                break;
            case VIRTIO_MMIO_QUEUE_DESC_LOW: {
                uint32_t qsel = dev->queue_sel;
                if (qsel < 2) {
                    dev->queues[qsel].desc_gpa = (dev->queues[qsel].desc_gpa & 0xffffffff00000000ULL) | val;
                }
                break;
            }
            case VIRTIO_MMIO_QUEUE_DESC_HIGH: {
                uint32_t qsel = dev->queue_sel;
                if (qsel < 2) {
                    dev->queues[qsel].desc_gpa = (dev->queues[qsel].desc_gpa & 0x00000000ffffffffULL) | ((uint64_t)val << 32);
                }
                break;
            }
            case VIRTIO_MMIO_QUEUE_AVAIL_LOW: {
                uint32_t qsel = dev->queue_sel;
                if (qsel < 2) {
                    dev->queues[qsel].avail_gpa = (dev->queues[qsel].avail_gpa & 0xffffffff00000000ULL) | val;
                }
                break;
            }
            case VIRTIO_MMIO_QUEUE_AVAIL_HIGH: {
                uint32_t qsel = dev->queue_sel;
                if (qsel < 2) {
                    dev->queues[qsel].avail_gpa = (dev->queues[qsel].avail_gpa & 0x00000000ffffffffULL) | ((uint64_t)val << 32);
                }
                break;
            }
            case VIRTIO_MMIO_QUEUE_USED_LOW: {
                uint32_t qsel = dev->queue_sel;
                if (qsel < 2) {
                    dev->queues[qsel].used_gpa = (dev->queues[qsel].used_gpa & 0xffffffff00000000ULL) | val;
                }
                break;
            }
            case VIRTIO_MMIO_QUEUE_USED_HIGH: {
                uint32_t qsel = dev->queue_sel;
                if (qsel < 2) {
                    dev->queues[qsel].used_gpa = (dev->queues[qsel].used_gpa & 0x00000000ffffffffULL) | ((uint64_t)val << 32);
                }
                break;
            }
            default:
                break;
        }
    } else {
        if (len != 4) return -EINVAL;
        uint32_t val = 0;

        switch (offset) {
            case VIRTIO_MMIO_MAGIC_VALUE:
                val = 0x74726976;
                break;
            case VIRTIO_MMIO_VERSION:
                val = 2;
                break;
            case VIRTIO_MMIO_DEVICE_ID:
                val = 3;
                break;
            case VIRTIO_MMIO_VENDOR_ID:
                val = 0x50414e45;
                break;
            case VIRTIO_MMIO_DEVICE_FEATURES:
                if (dev->device_features_sel == 1) {
                    val = 1; // VIRTIO_F_VERSION_1
                } else {
                    val = 0;
                }
                break;
            case VIRTIO_MMIO_QUEUE_NUM_MAX:
                if (dev->queue_sel < 2) {
                    val = 256;
                }
                break;
            case VIRTIO_MMIO_QUEUE_READY:
                if (dev->queue_sel < 2) {
                    val = dev->queues[dev->queue_sel].ready;
                }
                break;
            case VIRTIO_MMIO_INTERRUPT_STATUS:
                val = dev->interrupt_status;
                break;
            case VIRTIO_MMIO_STATUS:
                val = dev->status;
                break;
            default:
                break;
        }
        memcpy(data, &val, 4);
    }

    return 0;
}
