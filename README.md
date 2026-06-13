# PANE — A Lightweight VM Lifecycle Primitive

Pane is a **VM lifecycle primitive** — an embeddable tool, not a platform. The goal is to become the KVM control layer that Proxmox, E2B, GitHub Actions, and AI sandbox startups embed into their own products (similar to SQLite, but for VM management).

Pane is designed to perform five core operations faster and more reliably than any other VM management library on Linux.

---

## The Five Primitives (v0.1 Scope)

| Operation  | Description                                       | Latency Target     | Status      |
|------------|---------------------------------------------------|--------------------|-------------|
| `spawn`    | Boot a VM from an image                           | < 5ms (MicroVM)    | In Progress |
| `exec`     | Run command inside VM, return stdout/stderr/exit  | < 10ms round-trip  | Planned     |
| `snapshot` | Freeze RAM + CPU registers to disk                | < 100ms (4GB RAM)  | Planned     |
| `fork`     | CoW clone from snapshot, boot immediately         | < 50ms per VM      | Planned     |
| `destroy`  | Kill VM, reclaim all resources                    | < 50ms, zero leaks | **Done**    |

---

## Architecture

Pane is built with a three-layer architecture optimizing for native kernel APIs, memory safety, and embedding ergonomics:

```
┌──────────────────────────────────────────────┐
│             pane-cli  +  pane-api            │
│                      Go                      │
│   cobra CLI │ gRPC server │ CGo FFI to core  │
├──────────────────────────────────────────────┤
│              pane-core (orchestration)        │
│                     Rust                     │
│  VM state machines │ cgroups │ eBPF │ snaps  │
│  Calls into pane-vmm via FFI                 │
├──────────────────────────────────────────────┤
│               pane-vmm  (VMM layer)          │
│                      C                       │
│  /dev/kvm ioctls │ vCPU run loop │ io_uring  │
│  Guest memory mapping │ virtio setup         │
└──────────────────────────────────────────────┘
```

1. **C (`pane-vmm`)**: Low-overhead interaction with `/dev/kvm` ioctls and `liburing` for IO.
2. **Rust (`pane-core`)**: Safe, async orchestration of VM state machines, cgroups, and eBPF networks.
3. **Go (`pane-cli` / `pane-api`)**: Embedding-friendly CLI and gRPC server using standard cloud infrastructure toolchains.

---

## Directory Structure

```
pane/
├── CLAUDE.md                  # Claude Code master prompt & rules
├── README.md                  # Project-level overview & documentation
├── PLAN.md                    # Roadmap and progress checklist
├── TODO.md                    # Current phase TODO list
├── Makefile                   # Top-level make target runner
│
├── pane-vmm/                  # C, the KVM/io_uring VMM layer
│   ├── include/
│   │   └── pane_vmm.h         # Public C API header
│   ├── src/
│   │   ├── kvm.c              # VM creation, memory/vCPU setup, run loop
│   │   ├── virtio.c           # Virtio-MMIO console device emulation
│   │   └── test_*.c           # Integration/unit tests for the VMM
│   └── Makefile               # VMM build config
│
├── pane-core/                 # Rust, orchestration + eBPF + cgroups (TBD)
├── pane-api/                  # Go, gRPC server (TBD)
├── pane-cli/                  # Go, cobra CLI (TBD)
└── benchmarks/                # Criterion + Go benchmarks (TBD)
```

---

## Environment Requirements

- **OS**: Linux kernel 5.15+ (for stable io_uring and cgroup v2 support)
- **Arch**: x86_64
- **KVM**: `/dev/kvm` must exist and be accessible by the running user
- **C compiler**: GCC 12+ or Clang 15+

---

## Quick Start & Testing

To run the current VMM integration tests:

```bash
# Build and run basic VMM tests
make test

# Build and run the bare-metal serial boot test
./pane-vmm/test_boot_serial
```

See [pane-vmm/README.md](file:///home/varhan/projects/pane/pane-vmm/README.md) for detailed VMM documentation.
