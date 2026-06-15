#ifndef PANE_VMM_H
#define PANE_VMM_H

#include <stdint.h>
#include <stddef.h>
#include <stdbool.h>
#include <linux/kvm.h>

// Maximum number of memory slots supported
#define PANE_VMM_MAX_MEM_SLOTS 256

// Forward declaration
struct pane_vm;

// Opaque pointer to a VM instance
typedef struct pane_vm pane_vm_t;

// VM Backend Types
typedef enum {
    PANE_BACKEND_NATIVE = 0,
    PANE_BACKEND_QEMU
} pane_backend_t;

// VM Configuration Struct
typedef struct {
    const char *vm_id;
    const char *vmm_type;
    uint32_t vcpus;
    uint64_t memory_bytes;
    const char *disk_path;
    const char *disk_format;     /* "qcow2" | "raw" */
    bool virtio_net;
    bool virtio_blk;
    bool virtio_rng;
    const char *net_bridge;      /* may be NULL */
    const char *kernel_path;     /* may be NULL for direct-kernel-boot */
    const char *cmdline;         /* may be NULL */
    const char **extra_args;     /* NULL-terminated argv-style passthrough */
} pane_vmm_config_t;

// Create a new VM.
// Returns 0 on success, negative errno on failure.
// The caller owns the returned pointer and must call pane_vm_destroy when done.
int pane_vm_create(pane_vm_t **vm_out);

// Destroy a VM and free all associated resources.
void pane_vm_destroy(pane_vm_t *vm);

// Set user memory region for the VM.
// Returns 0 on success, negative errno on failure.
int pane_vm_set_user_memory_region(pane_vm_t *vm,
                                   uint32_t slot,
                                   uint64_t guest_phys_addr,
                                   uint64_t memory_size,
                                   uint64_t userspace_addr,
                                   uint32_t flags);

// Get the KVM file descriptor associated with the VM.
int pane_vm_get_kvm_fd(const pane_vm_t *vm);

// Get the VM file descriptor associated with the VM.
int pane_vm_get_vm_fd(const pane_vm_t *vm);

// Initialize the in-kernel IRQ chip (PIC/IOAPIC).
// Returns 0 on success, negative errno on failure.
int pane_vm_init_irqchip(pane_vm_t *vm);

// Create a vCPU with the specified ID.
// Returns 0 on success, negative errno on failure.
int pane_vm_vcpu_create(pane_vm_t *vm, uint32_t vcpu_id);

// Set general purpose registers for a vCPU.
// Returns 0 on success, negative errno on failure.
int pane_vm_vcpu_set_regs(pane_vm_t *vm, uint32_t vcpu_id, const struct kvm_regs *regs);

// Get general purpose registers for a vCPU.
// Returns 0 on success, negative errno on failure.
int pane_vm_vcpu_get_regs(const pane_vm_t *vm, uint32_t vcpu_id, struct kvm_regs *regs);

// Set special/segment registers for a vCPU.
// Returns 0 on success, negative errno on failure.
int pane_vm_vcpu_set_sregs(pane_vm_t *vm, uint32_t vcpu_id, const struct kvm_sregs *sregs);

// Get special/segment registers for a vCPU.
// Returns 0 on success, negative errno on failure.
int pane_vm_vcpu_get_sregs(const pane_vm_t *vm, uint32_t vcpu_id, struct kvm_sregs *sregs);

// Run a vCPU.
// Returns 0 on success, negative errno on failure.
int pane_vm_vcpu_run(pane_vm_t *vm, uint32_t vcpu_id);

// Get the vCPU file descriptor.
// Returns fd on success, negative errno on failure.
int pane_vm_get_vcpu_fd(const pane_vm_t *vm, uint32_t vcpu_id);

// Configure the VM for direct 64-bit boot (Firecracker Mode), mapping GDT and 4-level identity page tables.
// Returns 0 on success, negative errno on failure.
int pane_vm_setup_firecracker_mode(pane_vm_t *vm, uint32_t vcpu_id, uint64_t entry_point);

// Set up a virtio-mmio console device at the specified physical address range and IRQ.
// Returns 0 on success, negative errno on failure.
int pane_vm_setup_virtio_mmio(pane_vm_t *vm, uint64_t base_addr, uint64_t size, int irq);

// Set up a virtio-mmio block device at the specified physical address range, IRQ, and backing file path.
// Returns 0 on success, negative errno on failure.
int pane_vm_setup_virtio_blk(pane_vm_t *vm, uint64_t base_addr, uint64_t size, int irq, const char *disk_path);

// Set up a virtio-serial console for the VM.
// Returns 0 on success, negative errno on failure.
int pane_vm_set_virtio_console(pane_vm_t *vm);

// Get current VM backend type.
pane_backend_t pane_vm_get_backend(const pane_vm_t *vm);

// Configure the VM for QEMU mode, spawning QEMU with KVM acceleration and QMP socket.
// Returns 0 on success, negative errno on failure.
int pane_vm_setup_qemu_mode(pane_vm_t *vm, const pane_vmm_config_t *config, const char *qmp_socket_path);

// Suspend a QEMU VM.
// Returns 0 on success, negative errno on failure.
int pane_vm_qemu_suspend(pane_vm_t *vm);

// Resume a QEMU VM.
// Returns 0 on success, negative errno on failure.
int pane_vm_qemu_resume(pane_vm_t *vm);

// Query execution status of QEMU VM (returns e.g., "running", "paused").
// Returns 0 on success, negative errno on failure.
int pane_vm_qemu_query_status(pane_vm_t *vm, char *status_out, size_t max_len);

// Get the PID associated with the VM (QEMU PID or host process PID for native).
int pane_vm_get_pid(const pane_vm_t *vm);

#endif // PANE_VMM_H
