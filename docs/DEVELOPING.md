# Local Development Guide

This guide covers how to build, run, and debug Pane on a local Linux machine.

---

## Prerequisites

### Required

| Tool | Minimum version | Install |
|------|----------------|---------|
| Linux kernel | 5.8 | — |
| KVM | Any | `ls /dev/kvm` — must exist |
| Rust toolchain | 1.70 | `curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs \| sh` |
| Go | 1.21 | [go.dev/dl](https://go.dev/dl/) |
| Clang / LLVM | 14+ | `sudo apt install clang` / `sudo pacman -S clang` |
| liburing | 2.x | `sudo apt install liburing-dev` / `sudo pacman -S liburing` |
| Firecracker | 1.4+ | [github.com/firecracker-microvm/firecracker/releases](https://github.com/firecracker-microvm/firecracker/releases) |

### Optional (for eBPF development)

| Tool | Purpose |
|------|---------|
| `bpftool` | Inspect loaded BPF programs and maps |
| `bpftrace` | Ad-hoc BPF tracing scripts |
| `cargo-bpf` or `aya-tool` | Regenerate eBPF skeletons |

---

## Building

```bash
# Clone the repository
git clone https://github.com/g-varhan/Pane.git
cd Pane

# 1. Build C VMM static library
make -C pane-vmm

# 2. Build Rust core library
cd pane-core
cargo build --release
cd ..

# 3. Build Go gRPC server
cd pane-api
CGO_ENABLED=1 go build -o pane-api .
cd ..

# 4. (Optional) Build CLI
cd pane-cli
go build -o pane .
cd ..
```

### Quick build (all at once)

```bash
make          # delegates to sub-makefiles
```

---

## Running Tests

```bash
# C VMM tests
make -C pane-vmm test

# Rust unit + integration tests
cd pane-core && cargo test && cd ..

# Go tests
go test ./...
```

### Running Rust tests with logging

```bash
RUST_LOG=debug cargo test -- --nocapture 2>&1 | head -200
```

---

## Running `pane-api` Locally

```bash
# Ensure /run/pane exists and is writable (or use /tmp/pane fallback)
sudo mkdir -p /run/pane && sudo chown $USER /run/pane

# Start with debug logging
RUST_LOG=pane_core=debug,pane_api=debug ./pane-api/pane-api
```

The gRPC server listens on `localhost:50051` by default.

---

## Debugging

### Rust (`pane-core`)

Use `RUST_LOG` to control verbosity (see [logging conventions](internals/logging.md)):

```bash
RUST_LOG=pane_core=trace ./pane-api/pane-api
```

For interactive debugging with `rust-gdb` or `rust-lldb`:

```bash
cd pane-core
cargo build   # debug build (no --release)
rust-gdb target/debug/pane-api
```

### Go (`pane-api` / `pane-cli`)

Standard Go tooling works as expected:

```bash
# Delve debugger
dlv debug ./pane-api -- <flags>

# Race detector (always run in CI)
go test -race ./...
```

### C (`pane-vmm`)

```bash
# Build with debug symbols and AddressSanitizer
make -C pane-vmm CFLAGS="-g -O0 -fsanitize=address,undefined"

# Run under GDB
gdb ./pane-vmm/test_runner
```

### Inspecting cgroups

```bash
# List all Pane cgroups
ls /sys/fs/cgroup/pane/

# Check resource limits for a specific VM
cat /sys/fs/cgroup/pane/<vm-id>/memory.max
cat /sys/fs/cgroup/pane/<vm-id>/cpu.max
cat /sys/fs/cgroup/pane/<vm-id>/cgroup.procs
```

### Inspecting Firecracker

```bash
# Check if Firecracker API socket is up
curl --unix-socket /run/pane/fc-<vm-id>.sock http://localhost/

# Get VM state
curl --unix-socket /run/pane/fc-<vm-id>.sock http://localhost/vm
```

### Inspecting QMP (QEMU)

```bash
# Connect to QMP socket manually
nc -U /run/pane/qmp-<vm-id>.sock
# then type:
{"execute":"qmp_capabilities"}
{"execute":"query-status"}
```

### eBPF / Network

```bash
# List loaded BPF programs
sudo bpftool prog list

# Show maps
sudo bpftool map list

# Check TC filters on a TAP
tc filter show dev tap0 ingress
```

---

## Code Style

| Language | Formatter | Linter |
|----------|-----------|--------|
| Rust | `cargo fmt` | `cargo clippy -- -D warnings` |
| Go | `gofmt -w .` / `goimports -w .` | `go vet ./...` |
| C | `clang-format -i` (`.clang-format` in repo root) | — |

All formatters are run in CI. Unformatted code will fail the CI check.

---

## Environment Variables Reference

| Variable | Component | Default | Description |
|----------|-----------|---------|-------------|
| `RUST_LOG` | pane-core | `info` | Tracing filter (see [logging.md](internals/logging.md)) |
| `LOG_LEVEL` | pane-api | `info` | Go log verbosity (`debug`, `info`, `warn`, `error`) |
| `CGO_ENABLED` | pane-api build | `1` | Must be `1` to link FFI libraries |
| `PANE_AGENT_PATH` | pane-core | auto-detected | Override path to guest agent binary |
| `PANE_INIT_PATH` | pane-core | auto-detected | Override path to guest init binary |
| `PANE_VIRTIO_WIN_ISO` | pane-core | auto-detected | Override path to virtio-win ISO for Windows guests |

---

## Project Layout

```
Pane/
├── pane-vmm/        # C: KVM ioctls, virtio-mmio console, io_uring
├── pane-core/       # Rust: typestate VM management, eBPF, cgroups, vsock exec
│   └── src/
│       ├── vm.rs         # Vm<State> typestate machine
│       ├── resources.rs  # CgroupManager
│       ├── exec.rs       # Guest exec via vsock
│       ├── network.rs    # eBPF TC network isolation
│       ├── backends/     # Firecracker HTTP API client
│       └── ffi/          # CGo/FFI bindings for pane-vmm
├── pane-api/        # Go: gRPC server, CGo bindings
├── pane-cli/        # Go: CLI client
├── docs/
│   ├── adr/              # Architecture Decision Records
│   ├── internals/        # Resource lifecycle, VM state machine, logging
│   ├── TROUBLESHOOTING.md
│   └── DEVELOPING.md     # ← this file
└── packaging/       # Arch PKGBUILD, Fedora spec, Debian control
```
