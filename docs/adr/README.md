# Architecture Decision Records

This directory records significant architectural decisions made in Pane. Each
ADR captures the context, the decision, and its consequences. ADRs are
immutable once accepted; superseded decisions are marked accordingly.

## Index

| ADR | Title | Status |
|-----|-------|--------|
| [0001](0001-cgroups-v2.md) | Use cgroups v2 for VM Resource Isolation | Accepted |
| [0002](0002-typestate-vm.md) | Typestate Pattern for VM Lifecycle | Accepted |
| [0003](0003-dual-backend.md) | Dual VM Backend (Native KVM + Firecracker) | Accepted |
| [0004](0004-vsock-exec.md) | vsock + Unix Domain Socket for Guest Exec | Accepted |
| [0005](0005-ebpf-network.md) | eBPF Network Isolation via TC | Accepted |
| [0006](0006-cow-rootfs.md) | CoW Rootfs Cloning via Reflink | Accepted |

## Status Definitions

- **Accepted** — The decision is in force and implemented.
- **Proposed** — Under discussion; not yet implemented.
- **Deprecated** — Still in use but targeted for replacement; see the superseding ADR.
- **Superseded** — Replaced by a later ADR (link noted in the document).

## Adding a New ADR

Copy `0001-cgroups-v2.md` as a template. Number sequentially. Open a PR;
the ADR is merged once the corresponding implementation is merged.
