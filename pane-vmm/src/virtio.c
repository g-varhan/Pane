// SPDX-License-Identifier: Apache-2.0

#include "pane_vmm.h"
#include <errno.h>
#include <fcntl.h>
#include <pthread.h>
#include <linux/kvm.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/ioctl.h>
#include <unistd.h>

#include "pane_vmm_internal.h"

#define VIRTIO_MMIO_MAGIC_VALUE 0x000
#define VIRTIO_MMIO_VERSION 0x004
#define VIRTIO_MMIO_DEVICE_ID 0x008
#define VIRTIO_MMIO_VENDOR_ID 0x00c
#define VIRTIO_MMIO_DEVICE_FEATURES 0x010
#define VIRTIO_MMIO_DEVICE_FEATURES_SEL 0x014
#define VIRTIO_MMIO_DRIVER_FEATURES 0x020
#define VIRTIO_MMIO_DRIVER_FEATURES_SEL 0x024
#define VIRTIO_MMIO_QUEUE_SEL 0x030
#define VIRTIO_MMIO_QUEUE_NUM_MAX 0x034
#define VIRTIO_MMIO_QUEUE_NUM 0x038
#define VIRTIO_MMIO_QUEUE_READY 0x044
#define VIRTIO_MMIO_QUEUE_NOTIFY 0x050
#define VIRTIO_MMIO_INTERRUPT_STATUS 0x060
#define VIRTIO_MMIO_INTERRUPT_ACK 0x064
#define VIRTIO_MMIO_STATUS 0x070
#define VIRTIO_MMIO_QUEUE_DESC_LOW 0x080
#define VIRTIO_MMIO_QUEUE_DESC_HIGH 0x084
#define VIRTIO_MMIO_QUEUE_AVAIL_LOW 0x090
#define VIRTIO_MMIO_QUEUE_AVAIL_HIGH 0x094
#define VIRTIO_MMIO_QUEUE_USED_LOW 0x0a0
#define VIRTIO_MMIO_QUEUE_USED_HIGH 0x0a4

// Virtio descriptor flags
#define VIRTQ_DESC_F_NEXT 1
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
  struct virtio_mmio_dev base;
  pane_vm_t *vm;

  uint32_t device_features_sel;
  uint32_t driver_features_sel;
  uint32_t driver_features;
  uint32_t queue_sel;
  uint32_t interrupt_status;
  uint32_t status;

  struct virtio_queue queues[2]; // 0: RX, 1: TX
};

static int pane_console_handle_mmio(struct virtio_mmio_dev *dev_base,
                                    uint64_t phys_addr, uint8_t *data,
                                    uint32_t len, uint8_t is_write);

static void pane_console_free_dev(struct virtio_mmio_dev *dev_base) {
  free(dev_base);
}

int pane_virtio_mmio_init(pane_vm_t *vm, uint64_t base_addr, uint64_t size,
                          int irq) {
  struct virtio_mmio_console *dev =
      calloc(1, sizeof(struct virtio_mmio_console));
  if (!dev) {
    return -ENOMEM;
  }
  dev->base.base_addr = base_addr;
  dev->base.size = size;
  dev->base.irq = irq;
  dev->base.handle_mmio = pane_console_handle_mmio;
  dev->base.free_dev = pane_console_free_dev;

  dev->vm = vm;
  dev->status = 0;

  // Set default queue size max
  dev->queues[0].num = 256;
  dev->queues[1].num = 256;

  int ret = pane_vm_register_virtio_dev(vm, &dev->base);
  if (ret != 0) {
    free(dev);
    return ret;
  }
  return 0;
}

static void virtio_mmio_inject_irq(struct virtio_mmio_console *dev) {
  struct kvm_irq_level level = {
      .irq = dev->base.irq,
      .level = 1,
  };
  ioctl(pane_vm_get_vm_fd(dev->vm), KVM_IRQ_LINE, &level);
  level.level = 0;
  ioctl(pane_vm_get_vm_fd(dev->vm), KVM_IRQ_LINE, &level);
}

static void virtio_console_process_tx(struct virtio_mmio_console *dev) {
  struct virtio_queue *q = &dev->queues[1];
  if (!q->ready || q->desc_gpa == 0)
    return;

  struct virtq_desc *desc_table = pane_vm_gpa_to_hva(dev->vm, q->desc_gpa);
  struct virtq_avail *avail_ring = pane_vm_gpa_to_hva(dev->vm, q->avail_gpa);
  struct virtq_used *used_ring = pane_vm_gpa_to_hva(dev->vm, q->used_gpa);

  if (!desc_table || !avail_ring || !used_ring) {
    return;
  }

  while (q->last_avail_idx != avail_ring->idx) {
    uint16_t head = avail_ring->ring[q->last_avail_idx % q->num];
    if (head >= q->num) {
      q->last_avail_idx++;
      continue;
    }
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
      if (curr >= q->num) {
        break;
      }
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

static int pane_console_handle_mmio(struct virtio_mmio_dev *dev_base,
                                    uint64_t phys_addr, uint8_t *data,
                                    uint32_t len, uint8_t is_write) {
  struct virtio_mmio_console *dev = (struct virtio_mmio_console *)dev_base;
  uint64_t offset = phys_addr - dev_base->base_addr;

  if (is_write) {
    if (len != 4)
      return -EINVAL;
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
        dev->queues[qsel].desc_gpa =
            (dev->queues[qsel].desc_gpa & 0xffffffff00000000ULL) | val;
      }
      break;
    }
    case VIRTIO_MMIO_QUEUE_DESC_HIGH: {
      uint32_t qsel = dev->queue_sel;
      if (qsel < 2) {
        dev->queues[qsel].desc_gpa =
            (dev->queues[qsel].desc_gpa & 0x00000000ffffffffULL) |
            ((uint64_t)val << 32);
      }
      break;
    }
    case VIRTIO_MMIO_QUEUE_AVAIL_LOW: {
      uint32_t qsel = dev->queue_sel;
      if (qsel < 2) {
        dev->queues[qsel].avail_gpa =
            (dev->queues[qsel].avail_gpa & 0xffffffff00000000ULL) | val;
      }
      break;
    }
    case VIRTIO_MMIO_QUEUE_AVAIL_HIGH: {
      uint32_t qsel = dev->queue_sel;
      if (qsel < 2) {
        dev->queues[qsel].avail_gpa =
            (dev->queues[qsel].avail_gpa & 0x00000000ffffffffULL) |
            ((uint64_t)val << 32);
      }
      break;
    }
    case VIRTIO_MMIO_QUEUE_USED_LOW: {
      uint32_t qsel = dev->queue_sel;
      if (qsel < 2) {
        dev->queues[qsel].used_gpa =
            (dev->queues[qsel].used_gpa & 0xffffffff00000000ULL) | val;
      }
      break;
    }
    case VIRTIO_MMIO_QUEUE_USED_HIGH: {
      uint32_t qsel = dev->queue_sel;
      if (qsel < 2) {
        dev->queues[qsel].used_gpa =
            (dev->queues[qsel].used_gpa & 0x00000000ffffffffULL) |
            ((uint64_t)val << 32);
      }
      break;
    }
    default:
      break;
    }
  } else {
    if (len != 4)
      return -EINVAL;
    uint32_t val = 0;

    switch (offset) {
    case VIRTIO_MMIO_MAGIC_VALUE:
      val = 0x74726976;
      break;
    case VIRTIO_MMIO_VERSION:
      val = 2;
      break;
    case VIRTIO_MMIO_DEVICE_ID:
      val = 3; // Console device ID
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

int pane_handle_mmio(pane_vm_t *vm, uint64_t phys_addr, uint8_t *data,
                     uint32_t len, uint8_t is_write) {
  if (!vm)
    return -EINVAL;
  for (int i = 0; i < vm->virtio_dev_count; i++) {
    struct virtio_mmio_dev *dev = vm->virtio_devs[i];
    if (dev && phys_addr >= dev->base_addr &&
        phys_addr < dev->base_addr + dev->size) {
      return dev->handle_mmio(dev, phys_addr, data, len, is_write);
    }
  }
  return -ENOENT;
}

#define VIRTIO_BLK_T_IN 0
#define VIRTIO_BLK_T_OUT 1

#define VIRTIO_BLK_S_OK 0
#define VIRTIO_BLK_S_IOERR 1
#define VIRTIO_BLK_S_UNSUPP 2

struct virtio_blk_req_hdr {
  uint32_t type;
  uint32_t reserved;
  uint64_t sector;
} __attribute__((packed));

struct virtio_blk_req {
  struct virtio_mmio_blk *dev;
  uint16_t head;
  uint32_t data_len;
  uint8_t *status_buf;
};

struct virtio_mmio_blk {
  struct virtio_mmio_dev base;
  pane_vm_t *vm;
  int disk_fd;

  uint32_t device_features_sel;
  uint32_t driver_features_sel;
  uint32_t driver_features;
  uint32_t queue_sel;
  uint32_t interrupt_status;
  uint32_t status;

  struct virtio_queue queues[1]; // 0: request queue

  pthread_t completion_thread;
  int completion_thread_active;
  volatile int shutdown;
};

static void *virtio_blk_completion_thread(void *arg) {
  struct virtio_mmio_blk *dev = (struct virtio_mmio_blk *)arg;
  pane_vm_t *vm = dev->vm;
  struct io_uring_cqe *cqe;

  while (!dev->shutdown) {
    int ret = io_uring_wait_cqe(vm->ring, &cqe);
    if (ret < 0) {
      if (ret == -EINTR) continue;
      break;
    }
    if (cqe) {
      struct virtio_blk_req *req = io_uring_cqe_get_data(cqe);
      if (req) {
        struct virtio_queue *q = &dev->queues[0];

        *req->status_buf = (cqe->res >= 0) ? VIRTIO_BLK_S_OK : VIRTIO_BLK_S_IOERR;

        struct virtq_used *used_ring = pane_vm_gpa_to_hva(vm, q->used_gpa);
        if (used_ring) {
          uint32_t used_idx = used_ring->idx % q->num;
          used_ring->ring[used_idx].id = req->head;
          used_ring->ring[used_idx].len = req->data_len;
          __sync_synchronize();
          used_ring->idx++;
        }

        __sync_fetch_and_or(&dev->interrupt_status, 0x1);

        struct kvm_irq_level level = {
            .irq = dev->base.irq,
            .level = 1,
        };
        ioctl(pane_vm_get_vm_fd(vm), KVM_IRQ_LINE, &level);
        level.level = 0;
        ioctl(pane_vm_get_vm_fd(vm), KVM_IRQ_LINE, &level);

        free(req);
      }
      io_uring_cqe_seen(vm->ring, cqe);
    }
  }
  return NULL;
}

static void virtio_blk_process_queue(struct virtio_mmio_blk *dev) {
  struct virtio_queue *q = &dev->queues[0];
  if (!q->ready || q->desc_gpa == 0)
    return;

  struct virtq_desc *desc_table = pane_vm_gpa_to_hva(dev->vm, q->desc_gpa);
  struct virtq_avail *avail_ring = pane_vm_gpa_to_hva(dev->vm, q->avail_gpa);
  struct virtq_used *used_ring = pane_vm_gpa_to_hva(dev->vm, q->used_gpa);

  if (!desc_table || !avail_ring || !used_ring)
    return;

  int submitted = 0;
  int sync_completed = 0;

  while (q->last_avail_idx != avail_ring->idx) {
    uint16_t head = avail_ring->ring[q->last_avail_idx % q->num];
    if (head >= q->num) {
      q->last_avail_idx++;
      continue;
    }
    uint16_t curr = head;

    struct virtio_blk_req_hdr *hdr =
        pane_vm_gpa_to_hva(dev->vm, desc_table[curr].addr);
    if (!hdr || desc_table[curr].len < sizeof(struct virtio_blk_req_hdr)) {
      q->last_avail_idx++;
      continue;
    }

    if (!(desc_table[curr].flags & VIRTQ_DESC_F_NEXT)) {
      q->last_avail_idx++;
      continue;
    }
    curr = desc_table[curr].next;
    if (curr >= q->num) {
      q->last_avail_idx++;
      continue;
    }
    void *data_buf = pane_vm_gpa_to_hva(dev->vm, desc_table[curr].addr);
    uint32_t data_len = desc_table[curr].len;

    if (!(desc_table[curr].flags & VIRTQ_DESC_F_NEXT)) {
      q->last_avail_idx++;
      continue;
    }
    uint16_t status_desc_idx = desc_table[curr].next;
    if (status_desc_idx >= q->num) {
      q->last_avail_idx++;
      continue;
    }
    uint8_t *status_buf =
        pane_vm_gpa_to_hva(dev->vm, desc_table[status_desc_idx].addr);

    if (!data_buf || !status_buf) {
      q->last_avail_idx++;
      continue;
    }

    struct virtio_blk_req *req = malloc(sizeof(struct virtio_blk_req));
    if (!req) {
      *status_buf = VIRTIO_BLK_S_IOERR;
      uint32_t used_idx = used_ring->idx % q->num;
      used_ring->ring[used_idx].id = head;
      used_ring->ring[used_idx].len = 0;
      used_ring->idx++;
      sync_completed++;
      q->last_avail_idx++;
      continue;
    }

    req->dev = dev;
    req->head = head;
    req->data_len = data_len;
    req->status_buf = status_buf;

    uint64_t file_offset = hdr->sector * 512;
    int io_ret = 0;

    if (hdr->type == VIRTIO_BLK_T_IN) {
      extern int pane_uring_submit_read(pane_vm_t * vm, int fd, void *buf,
                                        uint32_t len, uint64_t offset,
                                        void *user_data);
      io_ret = pane_uring_submit_read(dev->vm, dev->disk_fd, data_buf, data_len,
                                      file_offset, req);
    } else if (hdr->type == VIRTIO_BLK_T_OUT) {
      extern int pane_uring_submit_write(pane_vm_t * vm, int fd,
                                         const void *buf, uint32_t len,
                                         uint64_t offset, void *user_data);
      io_ret = pane_uring_submit_write(dev->vm, dev->disk_fd, data_buf,
                                       data_len, file_offset, req);
    } else {
      io_ret = -ENOTSUP;
    }

    if (io_ret == 0) {
      submitted++;
    } else {
      *status_buf = (io_ret == -ENOTSUP) ? VIRTIO_BLK_S_UNSUPP : VIRTIO_BLK_S_IOERR;
      uint32_t used_idx = used_ring->idx % q->num;
      used_ring->ring[used_idx].id = head;
      used_ring->ring[used_idx].len = 0;
      used_ring->idx++;
      sync_completed++;
      free(req);
    }

    q->last_avail_idx++;
  }

  if (submitted > 0) {
    io_uring_submit(dev->vm->ring);
  }
  if (sync_completed > 0) {
    __sync_fetch_and_or(&dev->interrupt_status, 0x1);
    struct kvm_irq_level level = {
        .irq = dev->base.irq,
        .level = 1,
    };
    ioctl(pane_vm_get_vm_fd(dev->vm), KVM_IRQ_LINE, &level);
    level.level = 0;
    ioctl(pane_vm_get_vm_fd(dev->vm), KVM_IRQ_LINE, &level);
  }
}

static int pane_blk_handle_mmio(struct virtio_mmio_dev *dev_base,
                                uint64_t phys_addr, uint8_t *data, uint32_t len,
                                uint8_t is_write) {
  struct virtio_mmio_blk *dev = (struct virtio_mmio_blk *)dev_base;
  uint64_t offset = phys_addr - dev_base->base_addr;

  if (is_write) {
    if (len != 4)
      return -EINVAL;
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
      if (val == 0) {
        dev->queue_sel = val;
      }
      break;
    case VIRTIO_MMIO_QUEUE_NUM: {
      if (dev->queue_sel == 0) {
        dev->queues[0].num = val;
      }
      break;
    }
    case VIRTIO_MMIO_QUEUE_READY: {
      if (dev->queue_sel == 0) {
        dev->queues[0].ready = val;
      }
      break;
    }
    case VIRTIO_MMIO_QUEUE_NOTIFY:
      if (val == 0) {
        virtio_blk_process_queue(dev);
      }
      break;
    case VIRTIO_MMIO_INTERRUPT_ACK:
      __sync_fetch_and_and(&dev->interrupt_status, ~val);
      break;
    case VIRTIO_MMIO_STATUS:
      dev->status = val;
      if (val == 0) {
        memset(dev->queues, 0, sizeof(dev->queues));
        dev->queues[0].num = 256;
        dev->status = 0;
        dev->interrupt_status = 0;
      }
      break;
    case VIRTIO_MMIO_QUEUE_DESC_LOW: {
      if (dev->queue_sel == 0) {
        dev->queues[0].desc_gpa =
            (dev->queues[0].desc_gpa & 0xffffffff00000000ULL) | val;
      }
      break;
    }
    case VIRTIO_MMIO_QUEUE_DESC_HIGH: {
      if (dev->queue_sel == 0) {
        dev->queues[0].desc_gpa =
            (dev->queues[0].desc_gpa & 0x00000000ffffffffULL) |
            ((uint64_t)val << 32);
      }
      break;
    }
    case VIRTIO_MMIO_QUEUE_AVAIL_LOW: {
      if (dev->queue_sel == 0) {
        dev->queues[0].avail_gpa =
            (dev->queues[0].avail_gpa & 0xffffffff00000000ULL) | val;
      }
      break;
    }
    case VIRTIO_MMIO_QUEUE_AVAIL_HIGH: {
      if (dev->queue_sel == 0) {
        dev->queues[0].avail_gpa =
            (dev->queues[0].avail_gpa & 0x00000000ffffffffULL) |
            ((uint64_t)val << 32);
      }
      break;
    }
    case VIRTIO_MMIO_QUEUE_USED_LOW: {
      if (dev->queue_sel == 0) {
        dev->queues[0].used_gpa =
            (dev->queues[0].used_gpa & 0xffffffff00000000ULL) | val;
      }
      break;
    }
    case VIRTIO_MMIO_QUEUE_USED_HIGH: {
      if (dev->queue_sel == 0) {
        dev->queues[0].used_gpa =
            (dev->queues[0].used_gpa & 0x00000000ffffffffULL) |
            ((uint64_t)val << 32);
      }
      break;
    }
    default:
      break;
    }
  } else {
    if (len != 4)
      return -EINVAL;
    uint32_t val = 0;

    switch (offset) {
    case VIRTIO_MMIO_MAGIC_VALUE:
      val = 0x74726976;
      break;
    case VIRTIO_MMIO_VERSION:
      val = 2;
      break;
    case VIRTIO_MMIO_DEVICE_ID:
      val = 2; // Block device ID
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
      if (dev->queue_sel == 0) {
        val = 256;
      }
      break;
    case VIRTIO_MMIO_QUEUE_READY:
      if (dev->queue_sel == 0) {
        val = dev->queues[0].ready;
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

static void pane_blk_free_dev(struct virtio_mmio_dev *dev_base) {
  struct virtio_mmio_blk *dev = (struct virtio_mmio_blk *)dev_base;
  if (dev->completion_thread_active) {
    dev->shutdown = 1;
    struct io_uring_sqe *sqe = io_uring_get_sqe(dev->vm->ring);
    if (sqe) {
      io_uring_prep_nop(sqe);
      io_uring_sqe_set_data(sqe, NULL);
      io_uring_submit(dev->vm->ring);
    }
    pthread_join(dev->completion_thread, NULL);
  }
  if (dev->disk_fd >= 0) {
    close(dev->disk_fd);
  }
  free(dev);
}

int pane_vm_setup_virtio_blk(pane_vm_t *vm, uint64_t base_addr, uint64_t size,
                             int irq, const char *disk_path) {
  if (!vm || !disk_path)
    return -EINVAL;

  int fd = open(disk_path, O_RDWR | O_CLOEXEC);
  if (fd < 0) {
    return -errno;
  }

  extern int pane_uring_init(pane_vm_t * vm, uint32_t queue_depth);
  int ret = pane_uring_init(vm, 64);
  if (ret != 0) {
    close(fd);
    return ret;
  }

  struct virtio_mmio_blk *dev = calloc(1, sizeof(struct virtio_mmio_blk));
  if (!dev) {
    close(fd);
    return -ENOMEM;
  }

  dev->base.base_addr = base_addr;
  dev->base.size = size;
  dev->base.irq = irq;
  dev->base.handle_mmio = pane_blk_handle_mmio;
  dev->base.free_dev = pane_blk_free_dev;

  dev->vm = vm;
  dev->disk_fd = fd;
  dev->status = 0;
  dev->queues[0].num = 256;

  ret = pane_vm_register_virtio_dev(vm, &dev->base);
  if (ret != 0) {
    close(fd);
    free(dev);
    return ret;
  }

  dev->completion_thread_active = 0;
  dev->shutdown = 0;
  if (pthread_create(&dev->completion_thread, NULL, virtio_blk_completion_thread, dev) == 0) {
    dev->completion_thread_active = 1;
  }

  return 0;
}

int pane_vm_set_virtio_console(pane_vm_t *vm) {
  return pane_vm_setup_virtio_mmio(vm, 0x10000000, 512, 5);
}
