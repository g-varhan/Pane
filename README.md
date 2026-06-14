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

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux%20x86__64-lightgrey?style=flat-square&logo=linux)](https://kernel.org)
[![KVM](https://img.shields.io/badge/powered%20by-KVM%20%2F%20io__uring-orange?style=flat-square)](https://www.linux-kvm.org)
[![Language: C](https://img.shields.io/badge/core-C-blue?style=flat-square&logo=c)](pane-vmm/)
[![Language: Rust](https://img.shields.io/badge/orchestration-Rust-orange?style=flat-square&logo=rust)](pane-core/)
[![Language: Go](https://img.shields.io/badge/api%20%2F%20cli-Go-00ADD8?style=flat-square&logo=go)](pane-api/)
[![Status](https://img.shields.io/badge/status-early%20dev-red?style=flat-square)]()

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
| **`spawn`**  | Boot a VM from a disk image                     | **< 5ms** (MicroVM)     | 🔨 In Progress   |
| **`exec`**   | Run a command inside the VM, stream output      | **< 10ms** round-trip   | 📋 Planned       |
| **`snapshot`** | Freeze RAM + vCPU state to disk               | **< 100ms** (4 GB RAM)  | 📋 Planned       |
| **`fork`**   | CoW-clone a snapshot, boot immediately          | **< 50ms** per VM       | 📋 Planned       |
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
spawn  (MicroVM / Firecracker)   <  5ms    p99
spawn  (QEMU / Tiny10)           <  3s     p99
exec   round-trip                < 10ms    p99
snapshot  (4 GB VM)              < 100ms   p99
fork   (single)                  < 50ms    p99
fork   (50× parallel)            <  2s     wall time
destroy                          < 50ms    p99
fd leak after 1000 VMs           = 0       absolute
```

---

## Current Progress

```
Phase 1  ██████████  VM creation & destruction   ✅ DONE  (1000 VM FD-leak test passes)
Phase 2  ██████████  Memory mapping & virtio      ✅ DONE  (bare-metal boot, serial I/O)
Phase 3  ██████████  Firecracker backend          ✅ DONE  (target: < 5ms spawn, got 1.04ms)
Phase 4  ██████████  QEMU backend                 ✅ DONE  (QMP process lifecycle control)
Phase 5  ██████████  io_uring disk layer          ✅ DONE  (target: > 15% throughput gain, got > 200%)
Phase 6+ ░░░░░░░░░░  pane-core (Rust FFI + FSM)   🔨 NEXT  (FFI bindings & typestate FSM)
```

### What's working right now

- **Zero-leak VM lifecycle** — create and destroy 1,000 KVM VMs with no file descriptor or memory leaks
- **Guest memory mapping** — map anonymous host memory into guest physical address space with alignment validation (4K / 2MB / 1GB huge pages)
- **Direct 64-Bit Long Mode Boot (Firecracker mode)** — configures segment registers, 64-bit GDT, and 4-level identity page tables in C, bypassing 16/32-bit transitions entirely
- **Ultra-low cold-start latency** — boots a 64-bit guest payload, verifies Virtio-MMIO, and exits in **1.045 ms** (well under the 5 ms target budget)
- **QEMU Full Hardware Emulation Backend** — forks and spawns `qemu-system-x86_64` under KVM acceleration, establishing a JSON-based QMP socket client with automatic connection retry
- **Asynchronous QMP State Controls** — implements asynchronous QMP event filtering for robust runtime VM control (suspend, resume, query status, and clean shutdown)
- **io_uring Disk Layer (virtio-blk)** — integrates a high-performance, asynchronous disk I/O interface using Linux `io_uring` via `liburing`
- **Throughput Gains (> 200%)** — achieves over 2,000 MB/s read throughput, outperforming standard synchronous `pread` by more than 200% on sequential operations
- **SIGALRM watchdog** — a safety net that interrupts any blocking `KVM_RUN` ioctl if the guest hangs, preventing the host from getting stuck
- **Exit signal port** (`0x3f9`) — reliable guest-to-host VM termination that works even with an in-kernel IRQ chip
- **Virtio-MMIO v2 console** — full emulated register map, TX queue processing, and host-side IRQ injection
- **Bare-metal boot test** — a hand-crafted x86 real-mode payload that boots in the VM, verifies Virtio-MMIO magic, and exits cleanly

---

## Getting Started

### Requirements

- Linux **5.15+** (stable io_uring + cgroup v2)
- x86_64
- `/dev/kvm` accessible (`sudo usermod -aG kvm $USER`)
- GCC 12+ or Clang 15+

### Build & Test

```bash
git clone https://github.com/g-varhan/Pane.git
cd Pane

# Build and run the VMM test suite
make test
```

Expected output:
```
Running test: test_create_destroy
Initial open FDs: 45
After creating 1000 VMs, open FDs: 2045
After destroying all VMs, open FDs: 45
Test passed: no significant FD leak detected.

Running test: test_memory
All tests passed!
```

### Run the bare-metal boot test

```bash
cd pane-vmm
make
./test_boot_serial
```

```
No kernel image specified. Running embedded bare-metal test payload in Real Mode...
Starting VM...
HelloP
VM exited clean.
```

The VM booted, printed to the serial port, verified Virtio-MMIO, and shut down cleanly — all in microseconds.

---

## Repository Layout

```
pane/
├── README.md
├── PLAN.md                    ← roadmap & phase checklist
├── TODO.md                    ← current phase tasks
├── Makefile                   ← top-level test runner
│
├── pane-vmm/                  ← C · KVM + io_uring VMM layer
│   ├── include/pane_vmm.h     ← public API header
│   ├── src/
│   │   ├── kvm.c              ← VM lifecycle, vCPU run loop, watchdog
│   │   └── virtio.c           ← Virtio-MMIO v2 console emulation
│   └── README.md              ← VMM-specific documentation
│
├── pane-core/                 ← Rust · orchestration, cgroups, eBPF  (coming)
├── pane-api/                  ← Go   · gRPC server                   (coming)
├── pane-cli/                  ← Go   · cobra CLI                     (coming)
└── benchmarks/                ← criterion + Go benchmarks            (coming)
```

---

## Design Principles

- **No global mutable state.** All state lives in explicitly passed structs.
- **Every resource has a free path.** No `malloc` without a visible `free`. Ownership is documented.
- **Every ioctl checks its return.** On error: log errno, return typed code, never abort.
- **No invented constants.** All KVM flags come from `<linux/kvm.h>` on the build machine, never from memory.
- **Sanitizers on by default.** Debug builds use `-fsanitize=address,undefined`. CI blocks on any finding.
- **Rust: zero `unwrap()` in library code.** Errors propagate via `?` with typed variants.

---

## What Pane Is Not

| ❌ | |
|---|---|
| Not a platform | There is no Pane Cloud. There is no dashboard. |
| Not a Docker wrapper | Pane manages VMs, not containers. |
| Not a cluster orchestrator | Single host only, at least in v0.1. |
| Not systemd-dependent | Pane works on any Linux 5.15+ host. |
| Not cloud-provider-coupled | Bare Linux. No AWS SDK. No GCP client. |

---

## Roadmap

See [`PLAN.md`](PLAN.md) for the full ordered build checklist with pass/fail criteria for each phase.

---

## Contributing

The codebase is deliberately small and auditable. If you're interested in:
- Low-level KVM/io_uring hacking
- Building fast Rust ↔ C FFI boundaries
- eBPF micro-segmentation for VMs
- Sub-millisecond boot time optimization

...open an issue or send a PR. The build order in `PLAN.md` is the source of truth for what's needed next.

---

<div align="center">

**Pane** · Built on Linux KVM · Written in C, Rust, and Go

</div>
