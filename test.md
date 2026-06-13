# VMM Test Execution Proofs

This document contains the verified test execution logs and proofs for the KVM Virtual Machine Monitor (`pane-vmm`) layer of Pane.

---

## 1. Unified Test Suite (`make -C pane-vmm test`)

Runs the leak, memory boundary, 64-bit MicroVM, and QEMU QMP integration tests.

### Output

```
make: Entering directory '/home/varhan/projects/pane/pane-vmm'
Running test: test_create_destroy
Initial open FDs: 45
Created 1000 VMs
After creating 1000 VMs, open FDs: 2045
After destroying all VMs, open FDs: 45
Test passed: no significant FD leak detected.

Running test: test_memory
VM created successfully
KVM fd: 10, VM fd: 11
Memory region set successfully
Second memory region set successfully
Correctly rejected out of bounds slot
Correctly rejected null vm
All tests passed!

Running test: test_firecracker
Starting 64-bit MicroVM...
Hello from 64-bit Long Mode! P
VM exited clean.
Total VMM startup & execution latency: 1.470 ms
SUCCESS: Boot time is within target limits (1.470 ms < 5 ms)

Running test: test_qemu
Setting up dummy disk image...
Creating pane_vm...
Setting up QEMU mode...
Querying status (should be running)...
Status: running
Suspending VM...
Querying status (should be paused)...
Status: paused
Resuming VM...
Querying status (should be running)...
Status: running
Destroying VM...
Cleaning up disk image...
All QEMU backend tests passed!
make: Leaving directory '/home/varhan/projects/pane/pane-vmm'
```

---

## 2. Real-Mode Legacy Boot Test (`./pane-vmm/test_boot_serial`)

Tests basic I/O and register verification in 16-bit Real Mode.

### Output

```
No kernel image specified. Running embedded bare-metal test payload in Real Mode...
Starting VM...
HelloP
VM exited clean.
```

---

## Summary of Verified Metrics

- **File Descriptors**: **0 leaks** verified after spawning and destroying 1000 KVM VMs.
- **64-bit MicroVM Latency**: **1.470 ms** cold-start (initialization, GDT/Paging setup, execution, and exit), comfortably meeting the `< 5 ms` budget.
- **QEMU Backend**: Verified correct state transitions (`running` ➔ `paused` ➔ `running` ➔ `destroyed`) via QMP.
