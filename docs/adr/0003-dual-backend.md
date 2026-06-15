# ADR 0003 — Dual VM Backend (Native KVM + Firecracker)

| Field       | Value                |
|-------------|----------------------|
| Status      | Accepted             |
| Date        | 2024-01-01           |
| Deciders    | pane-vmm maintainers |
| Supersedes  | —                    |

## Context

Pane targets two distinct workloads:

1. **Low-latency microVM sandboxing** — short-lived containers, function
   execution; demands sub-100 ms boot and minimal memory overhead.
2. **Full-featured VM management** — Windows guests, GPU pass-through,
   USB, nested virtualisation; demands device richness.

No single hypervisor satisfies both requirements equally:

| Hypervisor | Boot time | Memory overhead | Device support | Snapshot/fork |
|-----------|-----------|----------------|----------------|---------------|
| QEMU/KVM  | ~500 ms   | ~20 MB          | Full           | `savevm` / live-migrate |
| Firecracker | <50 ms  | ~5 MB           | Minimal        | Diff snapshot + fork |

## Decision

`pane-core` exposes a **`VmBackend` enum** with two variants:

- `VmBackend::Firecracker(Box<FirecrackerVm>)` — wraps the Firecracker
  process, communicates via its local HTTP API socket.
- `VmBackend::Native(SafeVm)` — wraps `pane-vmm` (C layer) via FFI;
  supports QEMU/KVM and the custom `pane-vmm` KVM implementation.

All `Vm<State>` methods `match` on the backend and dispatch accordingly,
with no-op branches where a feature is unsupported for a given backend.

## Consequences

**Positive**
- Single `Vm<State>` API surface regardless of backend.
- Firecracker enables fast fork-based container cloning via
  `Vm::fork_firecracker()` and CoW rootfs (`cow_clone_rootfs()`).
- Native backend allows direct KVM ioctls and full device emulation.

**Negative**
- Feature parity is asymmetric: `exec()` via vsock works for both, but
  snapshotting in Native mode depends on QMP (`migrate exec:cat`), which
  requires QEMU rather than the custom `pane-vmm` kernel.
- Two distinct process management paths increases maintenance surface.

## References

- `pane-core/src/vm.rs` — `VmBackend`, `Vm::fork_firecracker()`
- `pane-core/src/backends/` — Firecracker API client
- `pane-vmm/` — native KVM backend (C)
