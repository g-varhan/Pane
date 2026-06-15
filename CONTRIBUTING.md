# Contributing to Pane

Thank you for your interest in contributing to Pane! Pane is designed to be a lightweight, embeddable hypervisor orchestration engine ("SQLite for VM management").

Please review the guidelines below to ensure a smooth contribution process.

---

## Environment & System Prerequisites

Because Pane directly interacts with low-level Linux hypervisor APIs, your development machine must meet the following capabilities:

- **OS**: Linux (x86_64) with Kernel version **5.15 or newer**.
- **KVM**: Direct hardware virtualization access (`/dev/kvm`). Ensure your user has read/write privileges (e.g., added to the `kvm` group).
- **liburing**: Ensure `liburing` headers and library are installed (typically `liburing-devel` or `liburing-dev`).
- **cgroups v2**: Pane relies exclusively on cgroups v2 resource limits (mounted at `/sys/fs/cgroup`).
- **CoW Filesystem**: Instant snapshot/fork cloning utilizes copy-on-write `reflink` capabilities. The backing image/instance directory must reside on a **Btrfs** or **XFS** mount (cloning will fail-fast on Ext4 or tmpfs).

---

## Build and Test Instructions

Before submitting a Pull Request, verify that all three components compile and pass their test suites cleanly.

### 1. VMM Layer (C)
Verify that the C core compiles under strict warning flags:
```bash
make -C pane-vmm clean
make -C pane-vmm all
make -C pane-vmm test
```

### 2. Orchestration Layer (Rust)
Ensure the Rust orchestration library builds and is warnings-free:
```bash
cd pane-core
cargo fmt --all --check
cargo clippy --all-targets -- -D warnings
cargo test
```

### 3. API Daemon & CLI (Go)
Validate the Go frontend and CGo FFI bindings:
```bash
go fmt ./...
go vet ./...
go test -v ./...
```

---

## Coding Standards

Pane maintains strict design and quality guidelines to preserve its minimal footprint and exceptional performance:

1. **Memory Safety & Unsafe Rust**:
   - Do **NOT** use `unwrap()` or `expect()` in library code (`pane-core`). All errors must propagate cleanly via `Result` and the `PaneError` enum.
   - Every `unsafe` block or `unsafe fn` must be accompanied by a `// SAFETY:` comment justifying why the block is correct and memory-safe.
2. **Device Emulation Rules**:
   - Always enforce **cgroups v2 only** for resource control.
   - eBPF filtering must use **Traffic Control (TC)** via the `aya` library. Do not introduce XDP or iptables fallbacks.
   - VMM processes must always be spawned via `execve` using an argv array. Never use `system()` or shell wrappers.
3. **No Unused Code**:
   - Keep the codebase lean. Avoid placeholders, dead code blocks, or active debug printing (`println!`, `printf`) in production code paths.
