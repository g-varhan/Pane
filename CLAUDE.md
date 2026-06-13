# PANE — Claude Code Master Prompt

## What You Are Building

Pane is a **VM lifecycle primitive** — an embeddable tool, not a platform.
The goal is not to compete with Proxmox or AWS. The goal is to become the
KVM control layer that Proxmox, E2B, GitHub Actions, and AI sandbox startups
embed into their own products. Think SQLite, but for VM management.

The entire value proposition rests on one thing: **Pane does five operations
faster and more reliably than anything else on Linux.** Nothing else matters
until those five are perfect.

---

## The Five Primitives (v0.1 scope — nothing else ships)

| Operation  | What it does                                      | Latency target     |
|------------|---------------------------------------------------|--------------------|
| `spawn`    | Boot a VM from an image                           | < 5ms (MicroVM), < 3s (QEMU) |
| `exec`     | Run command inside VM, return stdout/stderr/exit  | < 10ms round-trip  |
| `snapshot` | Freeze RAM + CPU registers to disk                | < 100ms (up to 4GB RAM) |
| `fork`     | CoW clone from snapshot, boot immediately         | < 50ms per VM      |
| `destroy`  | Kill VM, reclaim all resources                    | < 50ms, zero leaks |

Do not implement anything outside this list until all five pass their
benchmarks. No web UI, no clustering, no registry, no billing layer.

---

## Three-Layer Architecture

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

### Why three languages

**C (pane-vmm):** KVM was designed to be called from C. The ioctls, the
headers, the kernel documentation — all C-native. The VMM layer is ~15 core
functions, small enough to audit manually for memory safety. This is where
language overhead is physically measurable — one extra abstraction layer here
costs real nanoseconds. liburing (io_uring reference impl) is C. Use it
directly.

**Rust (pane-core):** The orchestration layer has complex shared mutable
state — dozens of async tasks observing and mutating VM state simultaneously.
This is exactly the problem Rust's ownership model was built to solve. Use
`aya` for eBPF (nothing else comes close outside of C). Use `tokio` for the
async runtime. Call into pane-vmm via a clean `extern "C"` boundary.

**Go (pane-cli + pane-api):** Every infrastructure tool the embedding
customers already use is Go — Terraform, Kubernetes, Docker. Single static
binary compilation. gRPC tooling is best-in-class. Goroutines handle
fan-out of simultaneous VM lifecycle events naturally. GC is acceptable here
because this layer has no sub-millisecond latency requirements.

---

## Repository Structure

```
pane/
├── CLAUDE.md                  ← this file
├── pane-vmm/                  ← C, the KVM/io_uring VMM layer
│   ├── include/
│   │   └── pane_vmm.h         ← public C header (also used by Rust FFI)
│   ├── src/
│   │   ├── kvm.c              ← /dev/kvm open, VM create, vCPU run loop
│   │   ├── memory.c           ← guest memory mapping, huge pages
│   │   ├── virtio.c           ← virtio-net, virtio-blk, virtio-serial
│   │   ├── uring.c            ← io_uring disk operations via liburing
│   │   └── backends/
│   │       ├── firecracker.c  ← MicroVM config (no legacy devices)
│   │       └── qemu.c         ← Full emulation config (Windows capable)
│   └── CMakeLists.txt
│
├── pane-core/                 ← Rust, orchestration + eBPF + cgroups
│   ├── src/
│   │   ├── lib.rs             ← public API — keep surface minimal and stable
│   │   ├── error.rs           ← typed errors via thiserror, never panic
│   │   ├── vm.rs              ← VM state machine (Spawning→Running→Frozen→Dead)
│   │   ├── snapshot.rs        ← CoW snapshot + fork logic
│   │   ├── resources.rs       ← cgroup v2: RAM balloon, CPU scheduling
│   │   ├── network.rs         ← eBPF routing via aya, WireGuard mesh
│   │   ├── exec.rs            ← vsock-based exec, stdout/stderr streaming
│   │   └── ffi/
│   │       └── vmm.rs         ← unsafe FFI bindings to pane-vmm C layer
│   ├── build.rs               ← cbindgen to generate C header for Go CGo
│   └── Cargo.toml
│
├── pane-api/                  ← Go, gRPC server + CGo into pane-core
│   ├── proto/
│   │   └── pane.proto         ← source of truth for all API contracts
│   ├── server/
│   │   └── handler.go
│   ├── ffi/
│   │   └── core.go            ← CGo bindings to pane-core Rust library
│   └── go.mod
│
├── pane-cli/                  ← Go, cobra CLI
│   ├── cmd/
│   │   ├── root.go
│   │   ├── run.go
│   │   ├── exec.go
│   │   ├── snapshot.go
│   │   ├── fork.go
│   │   └── destroy.go
│   └── go.mod
│
└── benchmarks/                ← criterion (Rust) + Go benchmarks
    ├── spawn_bench.rs
    └── fork_bench.go
```

---

## Build Order (do not skip steps)

Work top-to-bottom. Do not start a layer until the layer below it passes
its tests and benchmarks.

- [ ] **pane-vmm:** `pane_vm_create()`, `pane_vm_destroy()` — KVM fd open,
      basic VM struct, vCPU creation. No guest image yet. Test: create and
      destroy 1000 VMs without fd leak (check /proc/self/fd).

- [ ] **pane-vmm:** Guest memory mapping, virtio-serial setup. Test: boot
      a minimal Linux kernel (< 2MB bzImage) and get a console response.

- [ ] **pane-vmm:** Firecracker backend — strip all legacy devices, minimize
      device tree. Benchmark: boot time must hit < 5ms before proceeding.

- [ ] **pane-vmm:** QEMU backend — full hardware emulation, Windows-capable.
      Benchmark: boot time < 3s with Tiny10 image.

- [ ] **pane-vmm:** io_uring disk layer via liburing. Benchmark: sequential
      read throughput must exceed direct syscall baseline by > 15%.

- [ ] **pane-core:** FFI bindings to pane-vmm. Every C function wrapped with
      a typed Rust interface. No raw pointers visible above ffi/vmm.rs.

- [ ] **pane-core:** VM state machine. States: `Spawning → Running → Frozen
      → Dead`. Invalid transitions are compile-time errors via typestate
      pattern, not runtime panics.

- [ ] **pane-core:** `exec` via vsock. Stream stdout/stderr as async chunks.
      Benchmark: round-trip < 10ms on loopback.

- [ ] **pane-core:** snapshot + fork. Benchmark: fork 50 VMs from one
      snapshot in < 2 seconds total wall time.

- [ ] **pane-core:** cgroup v2 resource controls. RAM ballooning, CPU
      scheduling. Test: enforce 256MB RAM cap — guest OOM must not affect
      host.

- [ ] **pane-core:** eBPF network via aya. Micro-segmentation: two VMs in
      same group can reach each other, cannot reach VMs outside group.
      Zero iptables rules — verify with `iptables -L` returning empty.

- [ ] **pane-api:** Proto definitions. gRPC server skeleton. CGo bindings
      to pane-core.

- [ ] **pane-cli:** `pane run`, `pane exec`, `pane snapshot`, `pane fork`,
      `pane destroy`. Every command has `--json` flag.

---

## Non-Negotiable Rules Per Layer

### C (pane-vmm)

- Every function that returns a resource must have a corresponding free
  function. No exceptions. Document ownership in the header comment.
- Compile with `-Wall -Wextra -Werror -fsanitize=address,undefined` in
  debug builds. CI blocks on any sanitizer finding.
- Every `ioctl` call checks return value. On error: log the errno, return
  a typed error code, never abort.
- No `malloc` without a matching `free` path visible in the same function
  or clearly documented as transferred to caller.
- Use `__attribute__((cleanup))` for automatic fd cleanup where applicable.
- No global mutable state. All state lives in explicitly passed structs.

### Rust (pane-core)

- `cargo clippy -- -D warnings` must pass. Zero warnings, zero exceptions.
- `cargo fmt` enforced in CI.
- No `unwrap()` or `expect()` in library code. Every error must propagate
  via `?` with a typed error variant.
- Every `unsafe` block has a `// SAFETY:` comment explaining why it is
  sound. If you cannot write the comment, the unsafe block is wrong.
- Public API items (`pub fn`, `pub struct`) must have doc comments with
  a usage example.
- Async: `tokio` only. No `async-std`, no `smol`.
- eBPF programs live in `pane-core/src/bpf/` as separate `.bpf.c` files
  compiled by aya-build. Keep eBPF programs minimal — complex logic goes
  in Rust userspace.

### Go (pane-api + pane-cli)

- `golangci-lint run` must pass in CI.
- CGo surface must be minimal. Business logic lives in Go, not in C calls.
- All gRPC handlers have table-driven tests covering success + each error
  path.
- `--json` output on every CLI command is structured and stable — it is
  a public API contract from day one.
- Single binary: `CGO_ENABLED=1 go build -ldflags="-s -w"` must produce
  a self-contained binary. Verify with `ldd` — only libc dependency
  is acceptable.

---

## Performance Is a Hard Gate

These benchmarks run in CI. A regression blocks the merge. No exceptions,
no "we'll fix it later."

```
spawn (Firecracker)   < 5ms     p99
spawn (QEMU/Tiny10)   < 3s      p99  
exec round-trip       < 10ms    p99
snapshot (4GB VM)     < 100ms   p99
fork (single)         < 50ms    p99
fork (50x parallel)   < 2s      wall time
destroy               < 50ms    p99
fd leak after 1000    0         absolute
```

Benchmark tooling: `criterion` for Rust microbenchmarks. Go's
`testing.B` for Go layer. A bash script in `benchmarks/e2e.sh`
runs the full end-to-end suite against a real KVM instance.

---

## What Pane Is Not (enforce this strictly)

- **No web UI.** Ever. Not even "just for debugging."
- **No Docker dependency.** Pane does not wrap Docker.
- **No cluster orchestration in v0.1.** Single host only.
- **No container runtime.** Pane manages VMs. Containers are a different
  abstraction. Do not conflate them.
- **No cloud provider SDK dependencies.** Pane runs on bare Linux.
- **No systemd hard dependency.** Pane must work without systemd.

---

## Environment Assumptions

- Host OS: Linux 5.15+ (minimum for stable io_uring + cgroup v2)
- Architecture: x86_64 (arm64 is a future milestone, not v0.1)
- KVM must be available: `/dev/kvm` exists and is accessible
- Rust: 1.75 stable minimum
- Go: 1.22 minimum
- C: gcc 12+ or clang 15+
- liburing: 2.4+
- Root or appropriate capabilities for KVM and cgroup access

---

## When You Are Unsure About a Kernel Interface

Do not guess. Check in this order:

1. `man 2 ioctl_kvm` and related man pages
2. `linux/kvm.h` in kernel source
3. kernel.org documentation
4. QEMU or Firecracker source code for reference implementation

Never invent ioctl numbers or flag values. Always use the constants from
`<linux/kvm.h>` directly.

---

## Session Startup Checklist

At the start of every Claude Code session:

1. Run `cat CLAUDE.md` to reload full context
2. Run the test suite: `make test`
3. Run the benchmark suite: `make bench`
4. Check which build order item is next (the checklist above)
5. Work on exactly that item — nothing else

Do not context-switch between layers mid-session. Finish the current
checklist item, get it to green, commit, then move to the next.

## KVM Knowledge Rules

NEVER invent or recall ioctl constants from memory. Every KVM constant
must come from one of these sources, verified at write time:

- `/usr/include/linux/kvm.h` on the build machine (primary)
- https://github.com/torvalds/linux/blob/master/include/uapi/linux/kvm.h
- `kvm-ioctls` crate source for Rust wrappers

Required ioctl sequence for VM creation — verify against kernel source
before writing:
1. `KVM_CREATE_VM` on /dev/kvm fd
2. `KVM_SET_USER_MEMORY_REGION` to map guest RAM
3. `KVM_CREATE_VCPU` per vCPU
4. `KVM_SET_REGS` / `KVM_SET_SREGS` to set initial CPU state
5. `KVM_RUN` to enter guest

If unsure about any flag or struct field: STOP. Read the header.
Do not guess. Write a TODO comment and ask.

## io_uring Rules

- Use liburing (not raw syscalls) for all io_uring operations
- Minimum liburing version: 2.4
- Never use io_uring features above kernel 5.15 baseline without
  a runtime `io_uring_probe` capability check
- SQE field names must be verified against:
  `/usr/include/liburing/io_uring.h`
  NOT from memory
- Required setup sequence:
  1. `io_uring_queue_init(depth, &ring, flags)`
  2. Verify return value — negative errno on failure
  3. Get SQE: `io_uring_get_sqe(&ring)` — can return NULL if queue full
  4. Prep operation: `io_uring_prep_*(sqe, ...)`
  5. Submit: `io_uring_submit(&ring)`
  6. Wait: `io_uring_wait_cqe(&ring, &cqe)`
  7. Mark seen: `io_uring_cqe_seen(&ring, cqe)`
  Never skip step 7 — it leaks the completion queue entry.

  ## cgroup v2 Rules

Pane uses cgroup v2 exclusively. cgroup v1 interfaces must never appear.

Verify cgroup v2 is mounted: `mount | grep cgroup2`
Default mount: `/sys/fs/cgroup`

Key interfaces (verify paths on target system):
- Memory limit:  `memory.max`          (write bytes as integer)
- Memory soft:   `memory.high`
- CPU weight:    `cpu.weight`          (range 1-10000, default 100)
- CPU limit:     `cpu.max`             (format: "quota period" e.g. "50000 100000")
- PIDs limit:    `pids.max`
- Kill group:    `cgroup.kill`         (write "1" to kill all procs)

NEVER use:
- `memory.limit_in_bytes`  (v1 interface)
- `cpu.shares`             (v1 interface)
- `cpuset.cpus` without verifying v2 delegation

RAM ballooning sequence:
1. Write new limit to `memory.max`
2. If shrinking: write to `memory.high` first, wait for guest to reclaim
3. Kernel enforces — no ioctl needed

## eBPF / aya Rules

All eBPF code uses the `aya` crate. Never use libbpf-rs or raw bpf() syscalls.

eBPF program files live in: `pane-core/src/bpf/`
They are `.bpf.c` files compiled by `aya-build` in build.rs.

Correct aya map definition syntax:
```rust
#[map]
static ALLOWED_PAIRS: HashMap<u64, u8> = HashMap::with_max_entries(1024, 0);
```

Correct aya program attachment for TC (traffic control):
```rust
let program: &mut SchedClassifier = bpf.program_mut("pane_filter")
    .unwrap()
    .try_into()?;
program.load()?;
program.attach(&interface, TcAttachType::Egress)?;
```

Never use XDP for VM-to-VM traffic — TC is correct here.
XDP is for host-facing external interfaces only.

When writing BPF programs: keep them minimal. Any logic that can
live in Rust userspace should live there. BPF verifier rejections
are hard to debug — simple programs only.

## Firecracker Backend Rules

Firecracker is controlled via a Unix socket REST API, not CLI flags.
Pane spawns the Firecracker process then drives it via HTTP over the socket.

API version target: Firecracker 1.7+
Socket path convention: `/run/pane/fc-{vm_id}.sock`

Required boot sequence via API:
1. PUT /machine-config    (vCPU count, RAM, smt, track_dirty_pages)
2. PUT /boot-source       (kernel_image_path, boot_args)
3. PUT /drives/rootfs     (path_on_host, is_root_device, is_read_only)
4. PUT /network-interfaces/eth0  (iface_id, host_dev_name)
5. PUT /actions           ({"action_type": "InstanceStart"})

`track_dirty_pages: true` is REQUIRED for snapshot support.
Without it, snapshot will fail silently with incomplete state.

Snapshot API:
- CreateSnapshot: PUT /snapshot/create
  body: {"snapshot_type": "Full", "snapshot_path": "...", "mem_file_path": "..."}
- LoadSnapshot:   PUT /snapshot/load

Never pass kernel cmdline with `console=ttyS0` for production VMs —
it buffers and slows boot. Use `console=` (empty) or virtio console.

## vsock / exec Rules

Pane exec uses AF_VSOCK, not virtio-serial, not SSH.

vsock addressing:
- Host CID: 2 (always, kernel constant VMADDR_CID_HOST)
- Guest CID: assigned at VM creation, stored in vm.vsock_cid
- Port convention: 1024 for exec, 1025 for log streaming

Host-side server setup:
```c
int fd = socket(AF_VSOCK, SOCK_STREAM, 0);
struct sockaddr_vm addr = {
    .svm_family = AF_VSOCK,
    .svm_cid    = VMADDR_CID_ANY,
    .svm_port   = 1024,
};
bind(fd, (struct sockaddr*)&addr, sizeof(addr));
listen(fd, 128);
```

Guest-side agent is a minimal static binary compiled into the base image.
It listens on vsock port 1024, receives a JSON command envelope, exec()s
the command, streams stdout/stderr back, sends exit code.

Guest agent must be statically linked (no dynamic libc dependency).
Compile with: `gcc -static -o pane-agent agent.c`

## Snapshot and Fork Rules

Snapshot = RAM file + disk diff + CPU state JSON
Fork = new VM booted from snapshot's RAM file with CoW disk

Disk CoW uses Linux reflinks (not cp, not qcow2 internal CoW):
```bash
cp --reflink=always base.img fork-{id}.img
```
This is instantaneous and shares blocks until written.
Requires filesystem: btrfs or xfs with reflink support.
Ext4 does NOT support reflinks — fail fast with a clear error.

CPU state for Firecracker: captured by Firecracker snapshot API
CPU state for QEMU: use `QMP` (QEMU Machine Protocol) socket
  - Pause: `{"execute": "stop"}`  
  - Save state: `{"execute": "migrate", "arguments": {"uri": "exec:cat > state.bin"}}`
  - Never use QEMU's `-loadvm` flag — it requires monitor, use QMP

Fork sequence:
1. `cp --reflink=always` the snapshot disk image (instant)
2. Assign new vsock CID
3. Assign new tap interface
4. Boot new Firecracker/QEMU process pointing at forked disk + snapshot RAM
5. VM resumes from exact snapshot state in < 50ms
6. Update vm registry with new vm_id → process mapping

Memory: Firecracker snapshot RAM file is mmap'd into the new VM process.
The kernel handles CoW on the RAM pages automatically.
Do NOT copy the RAM file — mmap it with MAP_PRIVATE.