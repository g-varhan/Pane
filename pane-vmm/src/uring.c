// SPDX-License-Identifier: Apache-2.0

#include "pane_vmm_internal.h"
#include <errno.h>
#include <stdlib.h>
#include <string.h>

int pane_uring_init(pane_vm_t *vm, uint32_t queue_depth) {
  if (!vm)
    return -EINVAL;
  if (vm->ring)
    return 0; // Already initialized

  struct io_uring *ring = calloc(1, sizeof(struct io_uring));
  if (!ring) {
    return -ENOMEM;
  }

  int ret = io_uring_queue_init(queue_depth, ring, 0);
  if (ret < 0) {
    free(ring);
    return ret;
  }

  vm->ring = ring;
  return 0;
}

int pane_uring_submit_read(pane_vm_t *vm, int fd, void *buf, uint32_t len,
                           uint64_t offset, void *user_data) {
  if (!vm || !vm->ring)
    return -EINVAL;

  struct io_uring_sqe *sqe = io_uring_get_sqe(vm->ring);
  if (!sqe) {
    // Queue full, submit what we have and try again
    io_uring_submit(vm->ring);
    sqe = io_uring_get_sqe(vm->ring);
    if (!sqe) {
      return -EBUSY;
    }
  }

  io_uring_prep_read(sqe, fd, buf, len, offset);
  io_uring_sqe_set_data(sqe, user_data);

  return 0;
}

int pane_uring_submit_write(pane_vm_t *vm, int fd, const void *buf,
                            uint32_t len, uint64_t offset, void *user_data) {
  if (!vm || !vm->ring)
    return -EINVAL;

  struct io_uring_sqe *sqe = io_uring_get_sqe(vm->ring);
  if (!sqe) {
    io_uring_submit(vm->ring);
    sqe = io_uring_get_sqe(vm->ring);
    if (!sqe) {
      return -EBUSY;
    }
  }

  io_uring_prep_write(sqe, fd, buf, len, offset);
  io_uring_sqe_set_data(sqe, (void *)user_data);

  return 0;
}

int pane_uring_poll_completions(pane_vm_t *vm, void **user_data_out,
                                int32_t *res_out) {
  if (!vm || !vm->ring)
    return -EINVAL;

  struct io_uring_cqe *cqe = NULL;
  int ret = io_uring_peek_cqe(vm->ring, &cqe);
  if (ret == -EAGAIN || ret == -EINTR) {
    return 0; // No completions ready
  }
  if (ret < 0) {
    return ret;
  }

  if (cqe) {
    *user_data_out = io_uring_cqe_get_data(cqe);
    *res_out = cqe->res;
    io_uring_cqe_seen(vm->ring, cqe);
    return 1;
  }

  return 0;
}

// Clean up function called by kvm.c
void pane_uring_cleanup(struct io_uring *ring) {
  if (ring) {
    io_uring_queue_exit(ring);
  }
}
