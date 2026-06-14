# Phase 2: pane-vmm - Guest Memory Mapping & Virtio-Serial ✓

## Guest Memory Mapping
- [x] Implement pane_vm_set_user_memory_region() function
- [x] Add memory slot tracking to pane_vm struct
- [x] Handle memory alignment and huge page support
- [x] Add error checking for KVM_SET_USER_MEMORY_REGION ioctl
- [x] Implement memory cleanup in pane_vm_destroy()

## Virtio-Serial Setup
- [x] Research virtio-serial device requirements for KVM
- [x] Implement virtio-serial device creation functions
- [x] Configure virtio-serial backend in pane-vmm
- [x] Add virtio-serial pane to VM creation flow
- [x] Implement virtio-serial cleanup

## Integration & Testing
- [x] Create test that boots minimal Linux kernel (< 2MB bzImage)
- [x] Set up VM with mapped memory and virtio-serial
- [x] Implement console output reading via virtio-serial
- [x] Verify successful boot and console response
- [x] Ensure no resource leaks in test

## Dependencies & Tools Needed
- [x] Obtain or build minimal Linux kernel (bzImage < 2MB)
- [x] Consider using Buildroot or similar for minimal image
- [x] Verify libkvm headers are available

---

# Phase 3: pane-vmm - Firecracker Backend ✓

## MicroVM Configuration
- [x] Implement Direct Kernel Boot mechanism (loading vmlinux/ELF 64-bit kernel directly into memory)
- [x] Configure standard 64-bit flat memory mode registers (CS, DS, SS, ES, FS, GS, CR0, CR4, EFER, etc.)
- [x] Strip all legacy PC devices (no PIT, no PIC, no ACPI, no Floppy, etc.)
- [x] Implement zero-configuration boot (pass zeroed boot parameters / command line directly)

## Performance & Benchmarking
- [x] Implement high-resolution execution timing framework using `clock_gettime(CLOCK_MONOTONIC)`
- [x] Profile initialization vs guest execution time
- [x] Optimize memory mapping and vCPU initialization path
- [x] Benchmark and verify boot time < 5ms (P99)

---

# Phase 4: pane-vmm - QEMU Backend ✓

## QEMU Backend Setup
- [x] Implement QEMU configuration template (enabling full hardware emulation, multi-vCPU, standard BIOS/UEFI)
- [x] Configure PCI/PCIE bus layout and map standard devices (IDE/SATA, VGA/GOP display, Keyboard/Mouse)
- [x] Implement QMP (QEMU Machine Protocol) socket listener and parser
- [x] Implement QMP command envelope for VM state management (suspend, resume, query status)

## Guest OS Boot & Testing
- [x] Configure standard boot arguments for Windows (Tiny10) / Linux full installation images
- [x] Verify successful boot of a Tiny10 Windows image
- [x] Benchmark and verify boot time is < 3 seconds

---

# Phase 5: pane-vmm - io_uring Disk Layer ✓

## io_uring Integration
- [x] Implement `pane_uring_init` to initialize `io_uring` queue inside `pane_vm`
- [x] Implement `pane_uring_submit_read` and `pane_uring_submit_write` helpers using `liburing`
- [x] Implement non-blocking completion polling via `pane_uring_poll_completions`
- [x] Integrate `io_uring` read/write queue processing into `virtio-blk` device MMIO handlers
- [x] Verify correct file descriptor and queue resources cleanup on VM destroy

## Benchmarking & Performance
- [x] Create benchmark utility `test_uring_bench.c` compiling to compare synchronous vs `io_uring` reads
- [x] Verify sequential read throughput exceeds direct pread baseline by > 15% (achieved > 200% improvement)

---

# Phase 6: pane-core - FFI Bindings ✓

## FFI Interface Setup
- [x] Map C headers and exports to Rust `extern "C"` declarations in `ffi/vmm.rs`
- [x] Wrap raw pointers and unsafe C calls inside safe Rust abstractions
- [x] Propagate typed errors from VMM returns to Rust standard `Result` type

---

# Phase 7: pane-core - VM State Machine ✓

## State Machine
- [x] Define VM state types: Spawning, Running, Frozen, Dead
- [x] Implement sealed VmState trait to restrict invalid states
- [x] Implement Vm<State> generic over typestates
- [x] Implement compile-time enforced transition methods (start, freeze, resume, destroy)
- [x] Add doc comments and examples for all public elements
- [x] Create comprehensive integration test validating KVM lifecycle and Firecracker teardown
- [x] Fix clippy warnings and ensure warnings-free compilation

