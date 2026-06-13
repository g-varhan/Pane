# Pane Build Plan

Based on the build order specified in CLAUDE.md, we will implement Pane in the following phases:

## Phase 1: pane-vmm - Basic VM Creation/Destruction ✓
- [x] `pane_vm_create()`, `pane_vm_destroy()` — KVM fd open, basic VM struct, vCPU creation
- [x] Test: create and destroy 1000 VMs without fd leak (check /proc/self/fd)

## Phase 2: pane-vmm - Guest Memory Mapping & Virtio-Serial ✓
- [x] Guest memory mapping implementation
- [x] Virtio-serial device setup
- [x] Test: boot a minimal Linux kernel (< 2MB bzImage) and get a console response

## Phase 3: pane-vmm - Firecracker Backend ✓
- [x] Firecracker backend implementation (strip legacy devices, minimize device tree)
- [x] Benchmark: boot time must hit < 5ms

## Phase 4: pane-vmm - QEMU Backend ✓
- [x] QEMU backend implementation (full hardware emulation, Windows-capable)
- [x] Benchmark: boot time < 3s with Tiny10 image

## Phase 5: pane-vmm - io_uring Disk Layer
- [ ] io_uring disk layer via liburing implementation
- [ ] Benchmark: sequential read throughput must exceed direct syscall baseline by > 15%

## Phase 6: pane-core - FFI Bindings
- [ ] FFI bindings to pane-vmm (every C function wrapped with typed Rust interface)
- [ ] No raw pointers visible above ffi/vmm.rs

## Phase 7: pane-core - VM State Machine
- [ ] VM state machine: `Spawning → Running → Frozen → Dead`
- [ ] Invalid transitions are compile-time errors via typestate pattern

## Phase 8: pane-core - exec via vsock
- [ ] `exec` via vsock implementation
- [ ] Stream stdout/stderr as async chunks
- [ ] Benchmark: round-trip < 10ms on loopback

## Phase 9: pane-core - Snapshot + Fork
- [ ] Snapshot + fork implementation
- [ ] Benchmark: fork 50 VMs from one snapshot in < 2 seconds total wall time

## Phase 10: pane-core - cgroup v2 Resource Controls
- [ ] cgroup v2 resource controls implementation
- [ ] RAM ballooning, CPU scheduling
- [ ] Test: enforce 256MB RAM cap — guest OOM must not affect host

## Phase 11: pane-core - eBPF Network via aya
- [ ] eBPF network via aya implementation
- [ ] Micro-segmentation: two VMs in same group can reach each other, cannot reach VMs outside group
- [ ] Zero iptables rules — verify with `iptables -L` returning empty

## Phase 12: pane-api - gRPC Server
- [ ] Proto definitions
- [ ] gRPC server skeleton
- [ ] CGo bindings to pane-core

## Phase 13: pane-cli - CLI Commands
- [ ] `pane run`, `pane exec`, `pane snapshot`, `pane fork`, `pane destroy` commands
- [ ] Every command has `--json` flag

## Current Focus: Phase 5
