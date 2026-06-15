# pane-vmm

`pane-vmm` is the low-level Virtual Machine Monitor (VMM) layer of Pane, written in C. It interfaces directly with `/dev/kvm` via system ioctls to manage VM configuration, guest memory mapping, vCPUs, and minimal device emulation.

## Features

### 1. VM & vCPU Management
- **Creation/Destruction**: Simple FDs management for `/dev/kvm` and KVM VMs with thorough leak validation.
- **vCPU Controls**: Create vCPUs and configure general purpose registers (`struct kvm_regs`) and segment registers (`struct kvm_sregs`).
- **Signal-based Watchdog**: Safe execution wrapper via `SIGALRM` and `alarm(5)`. By disabling `SA_RESTART`, the watchdog interrupts blocking `ioctl(KVM_RUN)` system calls if a guest VM hangs or loops infinitely, preventing host resource exhaustion.

### 2. Guest Memory Mapping
- Maps host anonymous memory into the guest physical address space.
- Strictly enforces 4KB page alignment required by KVM.
- Validates huge page alignment constraints (2MB / 1GB) so that guest physical alignments match host userspace mappings.
- Supports up to 256 memory slots.

### 3. Emulated Hardware & I/O
- **Serial Out**: Emulates write-only output on standard PC serial port `0x3f8`. Output is flushed directly to the host's `stdout`.
- **Exit Signal Port**: Custom exit port `0x3f9` for guest-initiated VM shutdown. Writing to this port interrupts the run loop and triggers a clean host-side VM exit.
- **Virtio-MMIO v2 Console**: Emulated MMIO register map mapped at guest physical address `0x20000` (or `0x10000000` for kernel boot). Processes Virtio descriptors, handles TX queue notifications, writes payload data to host stdout, and injects virtual interrupts (`KVM_IRQ_LINE`) back to the guest.

---

## Code Map

- [pane_vmm.h](include/pane_vmm.h): The public API definition.
- [kvm.c](src/kvm.c): KVM initialization, memory mapping, vCPU setup, watchdog timer, and the vCPU run loop.
- [virtio.c](src/virtio.c): Virtio-MMIO register mapping and emulated serial console.
- [test_create_destroy.c](src/test_create_destroy.c): Verifies memory and FD sanity after spawning and destroying 1,000 VMs.
- [test_memory.c](src/test_memory.c): Validates alignment constraints and slot bounds.
- [test_boot_serial.c](src/test_boot_serial.c): Real-mode and protected-mode VM boot emulator.

---

## API Summary

```c
// VM Lifecycle
int pane_vm_create(pane_vm_t **vm_out);
void pane_vm_destroy(pane_vm_t *vm);

// Memory & Irqchip
int pane_vm_set_user_memory_region(pane_vm_t *vm, uint32_t slot, uint64_t gpa, uint64_t size, uint64_t hva, uint32_t flags);
int pane_vm_init_irqchip(pane_vm_t *vm);

// vCPU Operations
int pane_vm_vcpu_create(pane_vm_t *vm, uint32_t vcpu_id);
int pane_vm_vcpu_set_regs(pane_vm_t *vm, uint32_t vcpu_id, const struct kvm_regs *regs);
int pane_vm_vcpu_set_sregs(pane_vm_t *vm, uint32_t vcpu_id, const struct kvm_sregs *sregs);
int pane_vm_vcpu_run(pane_vm_t *vm, uint32_t vcpu_id);

// Virtio Device Setup
int pane_vm_setup_virtio_mmio(pane_vm_t *vm, uint64_t base_addr, uint64_t size, int irq);
```

---

## Building & Testing

To compile the VMM binaries:
```bash
cd pane-vmm
make
```

### Running Tests

1. **Unit & Leak Tests**:
   Runs `test_create_destroy` and `test_memory`.
   ```bash
   make test
   ```

2. **Boot Serial Integration Test**:
   ```bash
   ./test_boot_serial [path/to/bzImage]
   ```
   - **No Argument**: Runs a built-in 16-bit real-mode bare-metal payload. It prints `"Hello"`, reads the Virtio-MMIO magic header at `0x20000` (verifying it matches `"virt"`), prints `"P"` if successful, and triggers a clean exit via port `0x3f9`.
   - **With Argument**: Sets up a 32-bit protected-mode boot parameters block (`struct boot_params` / e820 table) and boots the specified minimal Linux kernel `bzImage`, routing boot parameters and command lines.
