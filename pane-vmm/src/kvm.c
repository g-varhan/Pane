#include "pane_vmm.h"
#include <linux/kvm.h>
#include <sys/ioctl.h>
#include <sys/mman.h>
#include <fcntl.h>
#include <unistd.h>
#include <errno.h>
#include <stdlib.h>
#include <stdio.h>
#include <time.h>
#include <signal.h>

#define PANE_VMM_MAX_MEM_SLOTS 256
#define PANE_VMM_MAX_VCPUS 32

// Forward declaration of MMIO handler implemented in virtio.c
int pane_handle_mmio(pane_vm_t *vm, uint64_t phys_addr, uint8_t *data, uint32_t len, uint8_t is_write);

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
};

int pane_vm_create(pane_vm_t **vm_out) {
    struct pane_vm *vm = calloc(1, sizeof(struct pane_vm));
    if (!vm) {
        return -ENOMEM;
    }

    // Open /dev/kvm
    vm->kvm_fd = open("/dev/kvm", O_RDWR | O_CLOEXEC);
    if (vm->kvm_fd < 0) {
        int err = errno;
        free(vm);
        return -err;
    }

    // Create a VM
    vm->vm_fd = ioctl(vm->kvm_fd, KVM_CREATE_VM, 0);
    if (vm->vm_fd < 0) {
        int err = errno;
        close(vm->kvm_fd);
        free(vm);
        return -err;
    }

    // Initialize memory slots to zero
    for (int i = 0; i < PANE_VMM_MAX_MEM_SLOTS; i++) {
        vm->mem_slots[i].slot = i;
        vm->mem_slots[i].memory_size = 0;
        vm->mem_slots[i].guest_phys_addr = 0;
        vm->mem_slots[i].userspace_addr = 0;
        vm->mem_slots[i].flags = 0;
    }

    // Initialize vCPU list to zero
    for (int i = 0; i < PANE_VMM_MAX_VCPUS; i++) {
        vm->vcpus[i].fd = -1;
        vm->vcpus[i].run = NULL;
    }

    *vm_out = vm;
    return 0;
}

void pane_vm_destroy(pane_vm_t *vm) {
    if (!vm) {
        return;
    }

    // Cleanup vCPUs
    for (int i = 0; i < PANE_VMM_MAX_VCPUS; i++) {
        if (vm->vcpus[i].fd >= 0) {
            if (vm->vcpus[i].run && vm->vcpu_mmap_size > 0) {
                munmap(vm->vcpus[i].run, vm->vcpu_mmap_size);
            }
            close(vm->vcpus[i].fd);
        }
    }

    if (vm->virtio_dev) {
        free(vm->virtio_dev);
    }

    if (vm->vm_fd >= 0) {
        close(vm->vm_fd);
    }
    if (vm->kvm_fd >= 0) {
        close(vm->kvm_fd);
    }
    free(vm);
}

int pane_vm_set_user_memory_region(pane_vm_t *vm,
                                   uint32_t slot,
                                   uint64_t guest_phys_addr,
                                   uint64_t memory_size,
                                   uint64_t userspace_addr,
                                   uint32_t flags) {
    if (!vm) {
        return -EINVAL;
    }
    if (slot >= PANE_VMM_MAX_MEM_SLOTS) {
        return -EINVAL;
    }

    // 4KB alignment is a hard requirement for KVM.
    if ((guest_phys_addr & 0xfff) != 0 ||
        (memory_size & 0xfff) != 0 ||
        (userspace_addr & 0xfff) != 0) {
        return -EINVAL;
    }

    // Huge page alignment support:
    // If the userspace mapping is aligned to 1GB or 2MB, and memory size is a multiple of that,
    // the guest physical address must be aligned to the same boundary.
    if ((userspace_addr & 0x3fffffff) == 0 && (memory_size & 0x3fffffff) == 0) {
        if ((guest_phys_addr & 0x3fffffff) != 0) {
            return -EINVAL; // 1GB huge page alignment mismatch
        }
    } else if ((userspace_addr & 0x1fffff) == 0 && (memory_size & 0x1fffff) == 0) {
        if ((guest_phys_addr & 0x1fffff) != 0) {
            return -EINVAL; // 2MB huge page alignment mismatch
        }
    }

    struct kvm_userspace_memory_region mem_region = {
        .slot = slot,
        .flags = flags,
        .guest_phys_addr = guest_phys_addr,
        .memory_size = memory_size,
        .userspace_addr = userspace_addr,
    };

    // Update our internal tracking
    vm->mem_slots[slot] = mem_region;

    // Call the KVM ioctl
    if (ioctl(vm->vm_fd, KVM_SET_USER_MEMORY_REGION, &mem_region) == -1) {
        return -errno;
    }

    return 0;
}

int pane_vm_get_kvm_fd(const pane_vm_t *vm) {
    if (!vm) {
        return -EINVAL;
    }
    return vm->kvm_fd;
}

int pane_vm_get_vm_fd(const pane_vm_t *vm) {
    if (!vm) {
        return -EINVAL;
    }
    return vm->vm_fd;
}

int pane_vm_get_vcpu_fd(const pane_vm_t *vm, uint32_t vcpu_id) {
    if (!vm || vcpu_id >= PANE_VMM_MAX_VCPUS) {
        return -EINVAL;
    }
    return vm->vcpus[vcpu_id].fd;
}

int pane_vm_init_irqchip(pane_vm_t *vm) {
    if (!vm) {
        return -EINVAL;
    }
    if (ioctl(vm->vm_fd, KVM_CREATE_IRQCHIP, 0) == -1) {
        return -errno;
    }
    // Create PIT2 with speaker-dummy flag to suppress periodic timer
    // interrupts that would otherwise wake the guest from HLT.
    struct kvm_pit_config pit_config = {
        .flags = KVM_PIT_SPEAKER_DUMMY,
    };
    if (ioctl(vm->vm_fd, KVM_CREATE_PIT2, &pit_config) == -1) {
        return -errno;
    }
    return 0;
}

int pane_vm_vcpu_create(pane_vm_t *vm, uint32_t vcpu_id) {
    if (!vm) {
        return -EINVAL;
    }
    if (vcpu_id >= PANE_VMM_MAX_VCPUS) {
        return -EINVAL;
    }
    if (vm->vcpus[vcpu_id].fd >= 0) {
        return -EEXIST;
    }

    if (vm->vcpu_mmap_size == 0) {
        int mmap_size = ioctl(vm->kvm_fd, KVM_GET_VCPU_MMAP_SIZE, 0);
        if (mmap_size < 0) {
            return -errno;
        }
        vm->vcpu_mmap_size = mmap_size;
    }

    int vcpu_fd = ioctl(vm->vm_fd, KVM_CREATE_VCPU, vcpu_id);
    if (vcpu_fd < 0) {
        return -errno;
    }

    struct kvm_run *run = mmap(NULL, vm->vcpu_mmap_size, PROT_READ | PROT_WRITE, MAP_SHARED, vcpu_fd, 0);
    if (run == MAP_FAILED) {
        int err = errno;
        close(vcpu_fd);
        return -err;
    }

    vm->vcpus[vcpu_id].fd = vcpu_fd;
    vm->vcpus[vcpu_id].run = run;
    vm->vcpus[vcpu_id].id = vcpu_id;
    vm->vcpu_count++;

    return 0;
}

int pane_vm_vcpu_set_regs(pane_vm_t *vm, uint32_t vcpu_id, const struct kvm_regs *regs) {
    if (!vm || vcpu_id >= PANE_VMM_MAX_VCPUS || vm->vcpus[vcpu_id].fd < 0) {
        return -EINVAL;
    }
    if (ioctl(vm->vcpus[vcpu_id].fd, KVM_SET_REGS, regs) == -1) {
        return -errno;
    }
    return 0;
}

int pane_vm_vcpu_get_regs(const pane_vm_t *vm, uint32_t vcpu_id, struct kvm_regs *regs) {
    if (!vm || vcpu_id >= PANE_VMM_MAX_VCPUS || vm->vcpus[vcpu_id].fd < 0) {
        return -EINVAL;
    }
    if (ioctl(vm->vcpus[vcpu_id].fd, KVM_GET_REGS, regs) == -1) {
        return -errno;
    }
    return 0;
}

int pane_vm_vcpu_set_sregs(pane_vm_t *vm, uint32_t vcpu_id, const struct kvm_sregs *sregs) {
    if (!vm || vcpu_id >= PANE_VMM_MAX_VCPUS || vm->vcpus[vcpu_id].fd < 0) {
        return -EINVAL;
    }
    if (ioctl(vm->vcpus[vcpu_id].fd, KVM_SET_SREGS, sregs) == -1) {
        return -errno;
    }
    return 0;
}

int pane_vm_vcpu_get_sregs(const pane_vm_t *vm, uint32_t vcpu_id, struct kvm_sregs *sregs) {
    if (!vm || vcpu_id >= PANE_VMM_MAX_VCPUS || vm->vcpus[vcpu_id].fd < 0) {
        return -EINVAL;
    }
    if (ioctl(vm->vcpus[vcpu_id].fd, KVM_GET_SREGS, sregs) == -1) {
        return -errno;
    }
    return 0;
}

// Signal handler for SIGALRM — empty, exists only to interrupt KVM_RUN.
static volatile sig_atomic_t pane_watchdog_fired = 0;

static void pane_watchdog_handler(int sig) {
    (void)sig;
    pane_watchdog_fired = 1;
}

int pane_vm_vcpu_run(pane_vm_t *vm, uint32_t vcpu_id) {
    if (!vm || vcpu_id >= PANE_VMM_MAX_VCPUS || vm->vcpus[vcpu_id].fd < 0) {
        return -EINVAL;
    }

    int vcpu_fd = vm->vcpus[vcpu_id].fd;
    struct kvm_run *run = vm->vcpus[vcpu_id].run;

    // Install SIGALRM handler and arm a 5-second watchdog.
    // SIGALRM interrupts the blocking KVM_RUN ioctl, guaranteeing
    // we never hang even if the guest enters an instruction loop
    // with no VM exits.
    pane_watchdog_fired = 0;
    struct sigaction sa = { .sa_handler = pane_watchdog_handler };
    sigemptyset(&sa.sa_mask);
    sa.sa_flags = 0; // Do NOT set SA_RESTART — we need EINTR.
    struct sigaction old_sa;
    sigaction(SIGALRM, &sa, &old_sa);
    alarm(5);

    int result = 0;

    while (1) {
        if (pane_watchdog_fired) {
            fprintf(stderr, "\n[WATCHDOG] vCPU execution timed out (safety limit reached)\n");
            result = -ETIME;
            break;
        }

        int ret = ioctl(vcpu_fd, KVM_RUN, 0);
        if (ret < 0) {
            if (errno == EINTR) {
                // SIGALRM (or another signal) interrupted KVM_RUN.
                // Loop back to check the watchdog flag.
                continue;
            }
            if (errno == EAGAIN) {
                continue;
            }
            result = -errno;
            break;
        }

        switch (run->exit_reason) {
            case KVM_EXIT_IO: {
                uint8_t *data = (uint8_t *)run + run->io.data_offset;
                if (run->io.port == 0x3f8 && run->io.direction == KVM_EXIT_IO_OUT) {
                    for (uint32_t i = 0; i < run->io.count; i++) {
                        putchar(data[i]);
                    }
                    fflush(stdout);
                } else if (run->io.port == 0x3f9 && run->io.direction == KVM_EXIT_IO_OUT) {
                    // Exit signal port: guest writes exit code here to stop.
                    result = 0;
                    goto cleanup;
                }
                break;
            }
            case KVM_EXIT_MMIO: {
                int mmio_ret = pane_handle_mmio(vm, run->mmio.phys_addr, run->mmio.data, run->mmio.len, run->mmio.is_write);
                if (mmio_ret < 0) {
                    fprintf(stderr, "MMIO emulation failed: address=0x%llx, error=%d\n",
                            run->mmio.phys_addr, mmio_ret);
                    result = mmio_ret;
                    goto cleanup;
                }
                break;
            }
            case KVM_EXIT_HLT:
                result = 0;
                goto cleanup;
            case KVM_EXIT_SHUTDOWN:
                result = 0;
                goto cleanup;
            case KVM_EXIT_FAIL_ENTRY:
                fprintf(stderr, "KVM_EXIT_FAIL_ENTRY: hardware_entry_failure_reason = 0x%llx\n",
                        run->fail_entry.hardware_entry_failure_reason);
                result = -EFAULT;
                goto cleanup;
            case KVM_EXIT_INTERNAL_ERROR:
                fprintf(stderr, "KVM_EXIT_INTERNAL_ERROR: suberror = 0x%x\n",
                        run->internal.suberror);
                result = -EFAULT;
                goto cleanup;
            default:
                fprintf(stderr, "Unhandled exit reason: %d\n", run->exit_reason);
                result = -EIO;
                goto cleanup;
        }
    }

cleanup:
    // Disarm the alarm and restore the previous signal handler.
    alarm(0);
    sigaction(SIGALRM, &old_sa, NULL);
    return result;
}

int pane_vm_setup_virtio_mmio(pane_vm_t *vm, uint64_t base_addr, uint64_t size, int irq) {
    extern int pane_virtio_mmio_init(pane_vm_t *vm, uint64_t base_addr, uint64_t size, int irq);
    return pane_virtio_mmio_init(vm, base_addr, size, irq);
}

void *pane_vm_get_virtio_dev(pane_vm_t *vm) {
    if (!vm) return NULL;
    return vm->virtio_dev;
}

void pane_vm_set_virtio_dev(pane_vm_t *vm, void *dev) {
    if (vm) {
        vm->virtio_dev = dev;
    }
}

void *pane_vm_gpa_to_hva(pane_vm_t *vm, uint64_t gpa) {
    if (!vm) return NULL;
    for (int i = 0; i < PANE_VMM_MAX_MEM_SLOTS; i++) {
        struct kvm_userspace_memory_region *slot = &vm->mem_slots[i];
        if (slot->memory_size > 0 &&
            gpa >= slot->guest_phys_addr &&
            gpa < slot->guest_phys_addr + slot->memory_size) {
            return (void *)(slot->userspace_addr + (gpa - slot->guest_phys_addr));
        }
    }
    return NULL;
}
