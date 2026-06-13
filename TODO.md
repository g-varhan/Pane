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

# Phase 3: pane-vmm - Firecracker Backend

## MicroVM Configuration
- [ ] Implement Direct Kernel Boot mechanism (loading vmlinux/ELF 64-bit kernel directly into memory)
- [ ] Configure standard 64-bit flat memory mode registers (CS, DS, SS, ES, FS, GS, CR0, CR4, EFER, etc.)
- [ ] Strip all legacy PC devices (no PIT, no PIC, no ACPI, no Floppy, etc.)
- [ ] Implement zero-configuration boot (pass zeroed boot parameters / command line directly)

## Performance & Benchmarking
- [ ] Implement high-resolution execution timing framework using `clock_gettime(CLOCK_MONOTONIC)`
- [ ] Profile initialization vs guest execution time
- [ ] Optimize memory mapping and vCPU initialization path
- [ ] Benchmark and verify boot time < 5ms (P99)
