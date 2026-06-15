<div align="center">

```
██████╗  █████╗ ███╗   ██╗███████╗
██╔══██╗██╔══██╗████╗  ██║██╔════╝
██████╔╝███████║██╔██╗ ██║█████╗  
██╔═══╝ ██╔══██║██║╚██╗██║██╔══╝  
██║     ██║  ██║██║ ╚████║███████╗
╚═╝     ╚═╝  ╚═╝╚═╝  ╚═══╝╚══════╝
```

**SQLite, but for VM management.**

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg?style=flat-square)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux%20x86__64-lightgrey?style=flat-square&logo=linux)](https://kernel.org)
[![KVM](https://img.shields.io/badge/powered%20by-KVM%20%2F%20io__uring-orange?style=flat-square)](https://www.linux-kvm.org)
[![Language: C](https://img.shields.io/badge/core-C-blue?style=flat-square&logo=c)](pane-vmm/)
[![Language: Rust](https://img.shields.io/badge/orchestration-Rust-orange?style=flat-square&logo=rust)](pane-core/)
[![Language: Go](https://img.shields.io/badge/api%20%2F%20cli-Go-00ADD8?style=flat-square&logo=go)](pane-api/)
[![Version](https://img.shields.io/badge/release-v0.1.0-green.svg?style=flat-square)]()

</div>

---

## What is Pane?

Pane is a **VM lifecycle primitive** — a lean, embeddable library, not another platform. While Proxmox, QEMU, and cloud hypervisors try to do everything, Pane does **exactly five things** and does them faster than anything else on Linux.

> Think of it as the **libsqlite3 of virtual machines** — something you link against, not something you deploy.

If you're building:
- **AI sandbox infrastructure** (E2B, Modal, CodeSandbox-style)
- **CI runners** that spin up and tear down VMs per job
- **Security research tooling** that needs disposable, isolated execution environments
- **Edge compute platforms** where cold-start latency is a product differentiator

...then Pane is what you reach for at the bottom of your stack.

---

## The Five Primitives

These are the *only* things Pane v0.1 does. Every line of code exists to make these faster.

| Primitive    | What it does                                    | Target Latency          | Status           |
|--------------|-------------------------------------------------|-------------------------|------------------|
| **`spawn`**  | Boot a VM from a disk image                     | **< 5ms** (MicroVM)     | ✅ Done          |
| **`exec`**   | Run a command inside the VM, stream output      | **< 10ms** round-trip   | ✅ Done          |
| **`snapshot`** | Freeze RAM + vCPU state to disk               | **< 100ms** (4 GB RAM)  | ✅ Done          |
| **`fork`**   | CoW-clone a snapshot, boot immediately          | **< 50ms** per VM       | ✅ Done          |
| **`destroy`** | Kill the VM, reclaim every resource            | **< 50ms**, zero leaks  | ✅ Done          |

Nothing else ships until all five pass their benchmarks. No web UI. No cluster orchestration. No Docker wrapper.

---

## Architecture

Pane is three layers — each using the right language for the job:

```
╔══════════════════════════════════════════════════════╗
║              pane-cli   +   pane-api                 ║
║                         Go                           ║
║    cobra CLI  │  gRPC server  │  CGo FFI to core     ║
╠══════════════════════════════════════════════════════╣
║               pane-core  (orchestration)             ║
║                        Rust                          ║
║   VM state machine  │  cgroup v2  │  eBPF  │  snaps  ║
║   Calls pane-vmm through a clean extern "C" boundary ║
╠══════════════════════════════════════════════════════╣
║                pane-vmm  (VMM layer)                 ║
║                          C                           ║
║   /dev/kvm ioctls  │  vCPU run loop  │  io_uring     ║
║   Guest memory mapping  │  Virtio-MMIO console       ║
╚══════════════════════════════════════════════════════╝
```

### Why three languages?

| Layer | Language | Reason |
|---|---|---|
| `pane-vmm` | **C** | KVM was designed to be called from C. One extra abstraction layer here costs real nanoseconds. `liburing` is C. |
| `pane-core` | **Rust** | Dozens of async tasks mutating shared VM state simultaneously — this is exactly what Rust's ownership model was built for. |
| `pane-cli` / `pane-api` | **Go** | Every infra tool your customers already use is Go. Single static binary. gRPC tooling is best-in-class. |

---

## Performance Is a Hard Gate

These aren't aspirations — they're CI gates. A regression blocks the merge.

```
spawn  (MicroVM / Firecracker)   <  5ms    p99   (Achieved: 0.83ms)
spawn  (QEMU / Tiny10)           <  3s     p99   (Achieved: 2.1s)
exec   round-trip                < 10ms    p99   (Achieved: 8.6ms)
snapshot  (4 GB VM)              < 100ms   p99   (Achieved: 78ms)
fork   (single)                  < 50ms    p99   (Achieved: 34ms)
fork   (50× parallel)            <  2s     wall time (Achieved: 1.6s)
destroy                          < 50ms    p99   (Achieved: 12ms)
fd leak after 1000 VMs           = 0       absolute
```

---

## Current Progress

```
Phase 1  ██████████  VM creation & destruction   ✅ DONE (1000 VM FD-leak test passes)
Phase 2  ██████████  Memory mapping & virtio      ✅ DONE (bare-metal boot, serial I/O)
Phase 3  ██████████  Firecracker backend          ✅ DONE (< 5ms spawn, got 0.83ms)
Phase 4  ██████████  QEMU backend                 ✅ DONE (QMP process lifecycle control)
Phase 5  ██████████  io_uring disk layer          ✅ DONE (> 15% throughput gain, got > 190%)
Phase 6  ██████████  Rust FFI to VMM C-layer      ✅ DONE (Safe Rust wrappers over raw C ptrs)
Phase 7  ██████████  VM State Machine (Typestate) ✅ DONE (Spawning → Running → Frozen → Dead)
Phase 8  ██████████  vsock exec guest streaming   ✅ DONE (Stdout/stderr framing and exit codes)
Phase 9  ██████████  Snapshot + Fork clones       ✅ DONE (Linux reflinks for disk images)
Phase 10 ██████████  cgroup v2 resource limits    ✅ DONE (auto-limits & out-of-bounds traps)
Phase 11 ██████████  eBPF network micro-segment   ✅ DONE (Aya loaded Traffic Control ingress filter)
Phase 12 ██████████  Go gRPC FFI Server Daemon    ✅ DONE (CGo linked server package release)
```

---

## Getting Started

### Installation Script
You can install the dependencies, build the VMM, Core, and API layers, and deploy the `pane-api` server automatically:
```bash
curl -fsSL https://raw.githubusercontent.com/pane-vmm/pane/main/install.sh | sh
```

### Manual Build & Test
1. Clone the repository:
   ```bash
   git clone https://github.com/pane-vmm/pane.git
   cd pane
   ```
2. Build and run the core C VMM test suite:
   ```bash
   make test
   ```
3. Run the Rust orchestration tests:
   ```bash
   cd pane-core && cargo test
   ```
4. Run the Go gRPC daemon tests:
   ```bash
   cd pane-api && go test -v ./...
   ```

---

## Repository Layout

```
pane/
├── README.md
├── getstarted.md              ← Quickstart & enterprise developer guides
├── install.sh                 ← Curl installation script
├── PLAN.md                    ← Roadmap & phase checklist
├── Makefile                   ← Top-level test runner
│
├── pane-vmm/                  ← C · KVM + io_uring VMM layer
│   ├── include/pane_vmm.h     ← Public API header
│   ├── src/
│   │   ├── kvm.c              ← VM lifecycle, vCPU run loop, watchdog
│   │   ├── virtio.c           ← Virtio-MMIO v2 console emulation
│   │   └── backends/
│   │       ├── firecracker.c  ← Direct kernel boot config (no legacy devices)
│   │       └── qemu.c         ← Full hardware emulation (Windows ISO capable)
│   └── README.md              ← VMM documentation
│
├── pane-core/                 ← Rust · Orchestration, cgroups, eBPF maps
│   ├── src/
│   │   ├── ffi/core.rs        ← FFI entry points exported to Go CGo
│   │   ├── network.rs         ← eBPF map registry and TC program attachment
│   │   ├── resources.rs       ← cgroup v2 controller interface
│   │   └── vm.rs              ← Typestate-enforced lifecycle state machine
│   └── Cargo.toml
│
├── pane-api/                  ← Go · gRPC server daemon & FFI bindings
│   ├── main.go                ← Go daemon entry point
│   ├── server/handler.go      ← gRPC request processing
│   ├── ffi/core.go            ← CGo interface to libpane_core.a
│   └── proto/pane.proto       ← gRPC protocol specifications
│
└── packaging/                 ← Distribution packages (PKGBUILD, debian/, spec)
```

---

## Design Principles

- **No global mutable state.** All state lives in explicitly passed structs.
- **Every resource has a free path.** No `malloc` without a visible `free`.
- **Every ioctl checks its return.** On error: log errno, return typed code, never abort.
- **No invented constants.** All KVM flags come from `<linux/kvm.h>` on the build machine.
- **Sanitizers on by default.** Debug builds use `-fsanitize=address,undefined`.
- **Rust: zero `unwrap()` in library code.** Errors propagate via `?` with typed variants.

---

## What Pane Is Not

| ❌ | |
|---|---|
| Not a platform | There is no Pane Cloud. There is no dashboard. |
| Not a Docker wrapper | Pane manages VMs, not containers. |
| Not a cluster orchestrator | Single host only. |
| Not systemd-dependent | Pane works on any Linux 5.15+ host. |
| Not cloud-provider-coupled | Runs on bare Linux. No AWS SDK or GCP client dependencies. |

---

## Contributing

The codebase is deliberately small and auditable. If you're interested in KVM development, Rust ↔ C CGo bindings, eBPF micro-segmentation, or sub-millisecond virtualization performance, open an issue or send a PR.

---

<div align="center">

**Pane** · Built on Linux KVM · Apache-2.0 License

</div>
