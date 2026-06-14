# Pane — Complete Reference Documentation

> **SQLite, but for VMs** — a lean, embeddable KVM lifecycle primitive for Linux x86_64.

---

## Table of Contents

1. [What is Pane?](#what-is-pane)
2. [Architecture](#architecture)
3. [Installation](#installation)
4. [Uninstallation](#uninstallation)
5. [Daemon Management](#daemon-management)
6. [Image Management](#image-management)
   - [pull](#pull) — Download a distro or custom image
   - [images](#images) — List pulled images
   - [image inspect](#image-inspect) — Inspect image config
   - [rmi](#rmi) — Remove an image
7. [VM Lifecycle](#vm-lifecycle)
   - [run](#run) — Boot a VM
   - [ps](#ps) — List running VMs
   - [exec](#exec) — Run a command inside a VM
   - [stop](#stop) — Gracefully stop a VM
   - [rm](#rm) — Destroy a VM
   - [logs](#logs) — View VM output
   - [inspect](#inspect) — Inspect VM details
8. [Snapshots & Cloning](#snapshots--cloning)
   - [snapshot](#snapshot) — Freeze VM state to disk
   - [fork](#fork) — Clone a snapshot into a new VM
9. [Configuration](#configuration)
   - [PaneSpec JSON](#panespec-json) — Full spec reference
   - [config validate](#config-validate)
   - [config show](#config-show)
10. [Linux Distros Reference](#linux-distros-reference)
11. [Windows (tiny10)](#windows-tiny10)
12. [Advanced Use Cases](#advanced-use-cases)
    - [Serverless Sandbox Infrastructure](#serverless-sandbox-infrastructure)
    - [CI Runners](#ci-runners)
    - [Security Research](#security-research)
    - [gRPC SDK Usage (Go)](#grpc-sdk-usage-go)
13. [Environment Variables](#environment-variables)
14. [Troubleshooting](#troubleshooting)
15. [Performance Benchmarks](#performance-benchmarks)

---

## What is Pane?

Pane is a **VM lifecycle primitive** — not a platform, not a cloud, not a dashboard. It does exactly **five things** at maximum speed:

| Primitive      | Description                              | Latency target        |
|----------------|------------------------------------------|-----------------------|
| **`spawn`**    | Boot a VM from an image                  | < 5 ms (MicroVM)      |
| **`exec`**     | Run a command inside a VM                | < 10 ms round-trip    |
| **`snapshot`** | Freeze RAM + vCPU state to disk          | < 100 ms (4 GB RAM)   |
| **`fork`**     | CoW-clone a snapshot into a new VM       | < 50 ms per clone     |
| **`destroy`**  | Kill a VM, reclaim every resource        | < 50 ms, zero leaks   |

If you're building AI sandboxes, CI runners, security research tooling, or edge compute — Pane is the bottom layer you reach for.

---

## Architecture

```
╔══════════════════════════════════════════════════════╗
║              pane-cli   +   pane-api                 ║  Go
║    cobra CLI  │  gRPC server  │  CGo FFI to core     ║
╠══════════════════════════════════════════════════════╣
║               pane-core  (orchestration)             ║  Rust
║   VM state machine  │  cgroup v2  │  eBPF  │  snaps  ║
╠══════════════════════════════════════════════════════╣
║                pane-vmm  (VMM layer)                 ║  C
║   /dev/kvm ioctls  │  vCPU run loop  │  io_uring     ║
╚══════════════════════════════════════════════════════╝
```

**pane-vmm** (C) — Direct KVM ioctls. No BIOS overhead. `liburing` disk I/O. Two backends: Firecracker (< 5 ms) and QEMU (full hardware emulation for Windows/desktop ISOs).

**pane-core** (Rust) — Typestate VM state machine, cgroup v2 resource limits, eBPF network micro-segmentation, snapshot/fork logic.

**pane-api / pane-cli** (Go) — gRPC daemon with CGo FFI into the Rust layer. Single static binary `pane` handles both the daemon and the CLI.

---

## Installation

### One-liner (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/g-varhan/Pane/main/install.sh | bash
```

### Options

```bash
# Custom install prefix
curl -fsSL .../install.sh | bash -s -- --prefix=/opt/pane

# Skip dep installation (if you already have them)
curl -fsSL .../install.sh | bash -s -- --skip-deps

# Don't install the systemd daemon
curl -fsSL .../install.sh | bash -s -- --no-daemon
```

### Manual build

```bash
git clone https://github.com/g-varhan/Pane.git
cd Pane
bash install.sh
```

### Requirements

| Requirement | Minimum | Notes |
|-------------|---------|-------|
| Linux kernel | 5.15+ | Tested on 6.x |
| Architecture | x86_64 | ARM support planned |
| KVM | Required | Enable Intel VT-x / AMD-V in BIOS |
| Go | 1.21+ | CGo must be enabled |
| Rust | 1.70+ | Stable toolchain |
| QEMU | 7.0+ | For Windows/ISO VMs |

### Supported distributions

| Distribution | Package manager | Status |
|---|---|---|
| Arch Linux | pacman | ✅ |
| Ubuntu 22.04 / 24.04 | apt | ✅ |
| Debian 12 (Bookworm) | apt | ✅ |
| Fedora 41 | dnf | ✅ |
| Rocky Linux 9 / AlmaLinux 9 | dnf | ✅ |
| openSUSE Leap 15.6 | zypper | ✅ |

---

## Uninstallation

```bash
# Preserve images and snapshots (default)
curl -fsSL https://raw.githubusercontent.com/g-varhan/Pane/main/uninstall.sh | bash

# Remove everything including VM images and snapshots
curl -fsSL .../uninstall.sh | bash -s -- --purge
```

Or locally:
```bash
bash uninstall.sh [--purge] [--prefix=/usr/local]
```

---

## Daemon Management

Pane operates as a gRPC daemon on a UNIX socket (`/run/pane.sock`). The CLI auto-starts it if needed, but you can manage it explicitly.

### `pane daemon start`

Start the Pane API daemon in the background.

```bash
pane daemon start

# Options:
#   --socket PATH   Use a custom UNIX socket path
#   --config PATH   Load a JSON config file
```

```bash
# Example: start daemon with systemd
sudo systemctl start pane

# Example: start manually in foreground (for debugging)
pane daemon start 2>&1 | tee /tmp/pane.log
```

### `pane daemon stop`

```bash
pane daemon stop
```

Gracefully sends SIGTERM to the daemon, waits for it to stop, then removes the socket file.

### `pane daemon status`

```bash
pane daemon status
# Output:
#   Daemon: running (PID 12345)
#   Socket: /run/pane.sock
#   Uptime: 3h 42m
```

### Systemd service

The installer creates `/etc/systemd/system/pane.service`. Use standard systemd commands:

```bash
sudo systemctl start   pane    # Start
sudo systemctl stop    pane    # Stop
sudo systemctl restart pane    # Restart
sudo systemctl enable  pane    # Auto-start on boot
sudo systemctl status  pane    # Status + recent logs
journalctl -u pane -f          # Follow logs
```

---

## Image Management

Images are stored in `/var/lib/pane/images/<name>/v1.0/`. Each image contains:
- `disk.iso` (or `disk.raw`, `disk.qcow2`) — the bootable disk
- `metadata.json` — name, version, source URL
- `panespec.json` — default VM configuration (CPUs, RAM, drivers)

---

### `pull`

Download a distribution ISO and register it as a Pane image.

```bash
pane pull <name-or-url>
```

#### Built-in Linux distros

All of these download from official mirrors with a live progress bar:

```bash
pane pull alpine             # Alpine Linux 3.21 (512 MiB RAM, 1 CPU)
pane pull ubuntu             # Ubuntu Server 24.04 LTS (2 GiB RAM, 2 CPUs)
pane pull ubuntu-desktop     # Ubuntu Desktop 24.04 LTS (4 GiB RAM, 2 CPUs)
pane pull ubuntu-minimal     # Ubuntu Server 24.04 — lightweight profile
pane pull debian             # Debian 12 Bookworm netinst (1 GiB RAM)
pane pull debian-live        # Debian 12 GNOME live (2 GiB RAM)
pane pull fedora             # Fedora Server 41 (2 GiB RAM, 2 CPUs)
pane pull fedora-workstation # Fedora Workstation 41 (4 GiB RAM)
pane pull arch               # Arch Linux (latest rolling release)
pane pull kali               # Kali Linux 2024.4 (2 GiB RAM)
pane pull rocky              # Rocky Linux 9.4 minimal
pane pull alma               # AlmaLinux 9.4 minimal
pane pull opensuse           # openSUSE Leap 15.6
```

While downloading, you'll see:
```
  Pulling ubuntu v24.04
  Source : https://releases.ubuntu.com/24.04/ubuntu-24.04.2-live-server-amd64.iso

  Connecting to https://releases.ubuntu.com/...
  [████████████░░░░░░░░░░░░░░░░░░]  42%  820.4 MiB / 1.9 GiB  18.2 MiB/s

✓  ubuntu (v24.04) pulled and registered successfully!
```

#### Custom HTTP(S) URL

```bash
pane pull https://example.com/custom-linux.iso
```

#### Windows (tiny10)

```bash
# Auto-downloads from archive.org (~2 GB) + VirtIO drivers from Fedora CDN
pane pull tiny10

# Use local ISO
TINY10_ISO_PATH=/path/to/tiny10.iso pane pull tiny10

# Use a custom URL (e.g. your GitHub Release asset)
GITHUB_TINY10_URL=https://github.com/you/pane-images/releases/download/v1/tiny10.iso \
  pane pull tiny10
```

See [Windows (tiny10)](#windows-tiny10) for full details.

---

### `images`

List all pulled images.

```bash
pane images
```

```
NAME              VERSION   VMM    SIZE     SOURCE
alpine            v3.21     qemu   196 MiB  https://dl-cdn.alpinelinux.org/...
ubuntu            v24.04    qemu   1.9 GiB  https://releases.ubuntu.com/...
tiny10            23H2      qemu   1.8 GiB  https://archive.org/...
```

---

### `image inspect`

Show the full `panespec.json` for an image — the default VM configuration that will be used when you `run` it.

```bash
pane image inspect ubuntu
pane image inspect tiny10
```

```json
{
  "vmm": "qemu",
  "cpus": 2,
  "memory": "2GiB",
  "disk": {
    "path": "/var/lib/pane/images/ubuntu/v1.0/disk.iso",
    "format": "raw"
  },
  "drivers": {
    "virtio_net": true,
    "virtio_blk": true
  }
}
```

---

### `rmi`

Remove a pulled image and free the disk space.

```bash
pane rmi alpine
pane rmi ubuntu
```

---

## VM Lifecycle

---

### `run`

Boot a VM from a pulled image or explicit parameters.

```bash
pane run [flags] <image>
```

#### Examples

```bash
# Simplest — use defaults from panespec.json
pane run alpine

# Override resources
pane run ubuntu --cpus 4 --memory 4GiB

# Boot with VNC display (for desktop ISOs)
pane run ubuntu-desktop --gui

# Assign a name (default: auto-generated ID)
pane run alpine --name my-sandbox

# Boot from a raw disk image on disk
pane run --iso /path/to/boot.iso --disk-size 20GiB

# Boot from a kernel directly (Firecracker/MicroVM mode)
pane run --kernel /boot/vmlinuz --cmdline "console=ttyS0 root=/dev/vda" alpine

# Use a PaneSpec config file
pane run -f /path/to/myvm.json

# Dry-run: print the resolved spec without starting
pane run --dry-run ubuntu
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--name` | string | auto | Human-readable VM name/ID |
| `--cpus` | uint | from image | Number of virtual CPUs |
| `--memory` | string | from image | RAM: `256MiB`, `2GiB`, etc. |
| `--disk-size` | string | — | Create a blank writable disk of this size |
| `--iso` | string | — | Path to an ISO to boot from |
| `--kernel` | string | — | Path to a Linux kernel (enables Firecracker mode) |
| `--cmdline` | string | — | Kernel command-line arguments |
| `--gui` | bool | false | Enable VNC display (`:1` → port 5901) |
| `--display` | string | none | `vnc`, `sdl`, `none` |
| `--no-virtio-net` | bool | false | Disable VirtIO network |
| `--no-virtio-blk` | bool | false | Disable VirtIO block |
| `-f, --config` | string | — | Load a PaneSpec JSON config file |
| `--dry-run` | bool | false | Print resolved spec, don't boot |
| `--standalone` | bool | false | Bypass daemon (direct FFI) |

---

### `ps`

List all running VMs.

```bash
pane ps
```

```
ID              IMAGE    STATUS   CPUs  MEMORY  PID    UPTIME
my-sandbox      alpine   running  1     512MiB  18432  5m 12s
build-runner-1  ubuntu   running  4     4GiB    18891  2m 04s
```

---

### `exec`

Run a command inside a running VM and stream output back.

```bash
pane exec <vm-id> <command> [args...]
```

```bash
pane exec my-sandbox uname -a
pane exec my-sandbox sh -c "apt update && apt install -y curl"
pane exec my-sandbox cat /etc/os-release

# Pipe stdin into the command
echo "hello" | pane exec my-sandbox cat -
```

Connects via vsock. Round-trip latency: < 10 ms p99.

---

### `stop`

Gracefully stop a running VM (SIGTERM → SIGKILL after timeout).

```bash
pane stop <vm-id>
pane stop my-sandbox
```

---

### `rm`

Destroy a VM and reclaim all resources (memory, cgroup, network).

```bash
pane rm <vm-id>
pane rm my-sandbox
```

> **Note:** `rm` does not delete the image. Use `rmi` for that.

---

### `logs`

View the serial console / stdout output of a VM.

```bash
pane logs <vm-id>
pane logs my-sandbox

# Follow (like tail -f)
pane logs -f my-sandbox
```

---

### `inspect`

Show the full runtime state of a running VM.

```bash
pane inspect <vm-id>
```

```json
{
  "id": "my-sandbox",
  "image": "alpine",
  "pid": 18432,
  "vsock_cid": 4,
  "status": "running",
  "cpus": 1,
  "memory": "512MiB",
  "qmp_socket": "/run/pane/my-sandbox.qmp",
  "uptime_seconds": 312
}
```

---

## Snapshots & Cloning

---

### `snapshot`

Freeze a running VM's RAM + vCPU state to disk. The VM is paused.

```bash
pane snapshot <vm-id>
pane snapshot my-sandbox
```

Snapshots are stored in `/var/lib/pane/snapshots/<vm-id>/`. They include:
- `ram.img` — complete guest memory dump
- `cpu.state` — serialized vCPU registers
- `disk.cow` — copy-on-write disk overlay

Snapshot time: **< 100 ms** for a 4 GB VM.

To resume a snapshot:
```bash
pane run --snapshot /var/lib/pane/snapshots/my-sandbox alpine
```

---

### `fork`

Clone a snapshot into one or more new VMs. Uses Linux reflinks (CoW) — disk is shared until writes diverge.

```bash
pane fork <source-vm-id> <new-vm-id>
pane fork my-sandbox clone-1

# Fork multiple clones at once
pane fork my-sandbox clone-1 clone-2 clone-3

# Fork 50 clones in parallel (under 2 seconds wall time)
for i in $(seq 1 50); do
  pane fork my-sandbox worker-$i &
done
wait
```

Clone latency: **< 50 ms** per VM. 50 clones in **< 2 s** total.

---

## Configuration

---

### PaneSpec JSON

Every `run` command uses a `PaneSpec`. You can provide a full spec file with `-f`:

```bash
pane run -f myvm.json
```

#### Complete PaneSpec reference

```jsonc
{
  // VMM backend: "qemu" (full emulation) or "firecracker" (MicroVM, Linux only)
  "vmm": "qemu",

  // Virtual CPU count
  "cpus": 2,

  // Guest RAM: supports MiB / GiB suffixes
  "memory": "2GiB",

  // Boot disk
  "disk": {
    "path": "/var/lib/pane/images/ubuntu/v1.0/disk.iso",
    "format": "raw",       // "raw", "qcow2"
    "size": "20GiB"        // only if creating a new writable disk
  },

  // VirtIO drivers
  "drivers": {
    "virtio_net": true,
    "virtio_blk": true,
    "virtio_rng": false
  },

  // Network
  "network": {
    "tap_device": "pane0",
    "mac": "52:54:00:12:34:56"   // auto-generated if omitted
  },

  // cgroup v2 resource limits
  "resources": {
    "cpu_quota": 50,             // % of one CPU core
    "memory_max": "2GiB",
    "pids_max": 512
  },

  // Pass extra QEMU flags directly (advanced)
  "extra_args": ["-vnc", ":1", "-cdrom", "/path/to/drivers.iso"],

  // Firecracker-only: direct kernel boot
  "kernel": {
    "path": "/boot/vmlinuz",
    "cmdline": "console=ttyS0 reboot=k panic=1 pci=off"
  }
}
```

---

### `config validate`

Validate a PaneSpec JSON file.

```bash
pane config validate myvm.json
# Output:
#   ✓ myvm.json is valid
#   CPUs: 2, Memory: 2GiB, VMM: qemu
```

---

### `config show`

Print the default PaneSpec profile.

```bash
pane config show
```

---

## Linux Distros Reference

| Name | Version | RAM | CPUs | Source mirror |
|------|---------|-----|------|---------------|
| `alpine` | 3.21 | 512 MiB | 1 | dl-cdn.alpinelinux.org |
| `ubuntu` | 24.04 LTS | 2 GiB | 2 | releases.ubuntu.com |
| `ubuntu-desktop` | 24.04 LTS | 4 GiB | 2 | releases.ubuntu.com |
| `ubuntu-minimal` | 24.04 LTS | 1 GiB | 1 | releases.ubuntu.com |
| `debian` | 12 Bookworm | 1 GiB | 1 | cdimage.debian.org |
| `debian-live` | 12 Bookworm GNOME | 2 GiB | 2 | cdimage.debian.org |
| `fedora` | 41 Server | 2 GiB | 2 | download.fedoraproject.org |
| `fedora-workstation` | 41 | 4 GiB | 2 | download.fedoraproject.org |
| `arch` | rolling | 1 GiB | 1 | geo.mirror.pkgbuild.com |
| `kali` | 2024.4 | 2 GiB | 2 | cdimage.kali.org |
| `rocky` | 9.4 minimal | 2 GiB | 2 | download.rockylinux.org |
| `alma` | 9.4 minimal | 2 GiB | 2 | repo.almalinux.org |
| `opensuse` | Leap 15.6 | 2 GiB | 2 | download.opensuse.org |

All distro URLs are baked into the binary. Override any URL with the `PANE_IMAGE_URL_<NAME>` env var (future feature) or use the generic HTTP pull.

---

## Windows (tiny10)

tiny10 is a minimal Windows 11 23H2 image (~1.8 GB). Pane automatically:
1. Downloads tiny10 from the Internet Archive
2. Downloads VirtIO-Win drivers from Fedora CDN
3. Sets up a QEMU VM with VNC display (port 5901)

```bash
pane pull tiny10     # ~4 GB total download (tiny10 + VirtIO drivers)
pane run tiny10      # Boot — connect VNC viewer to localhost:5901
```

### Resolution priority for tiny10 ISO

| Priority | Source | How to set |
|----------|--------|------------|
| 1 | Custom GitHub Release URL | `GITHUB_TINY10_URL=https://...` |
| 2 | Custom local path | `TINY10_ISO_PATH=/path/to/tiny10.iso` |
| 3 | Default local path | `/home/<user>/Documents/disk/tiny10 x64 23h2.iso` |
| 4 | Internet Archive (auto) | `https://archive.org/download/tiny-10-23-h2/...` |

### Resolution priority for VirtIO drivers

| Priority | Source | How to set |
|----------|--------|------------|
| 1 | Custom ISO path | `VIRTIO_WIN_ISO=/path/to/virtio-win.iso` |
| 2 | Custom URL | `VIRTIO_WIN_URL=https://...` |
| 3 | Default local path | `/home/<user>/Documents/disk/virtio-win-*.iso` |
| 4 | Fedora CDN (auto) | `https://fedorapeople.org/.../virtio-win-0.1.285.iso` |

### Hosting tiny10 on GitHub (for teams)

Upload your ISO as a GitHub Release asset:

```bash
# Create release and upload
gh release create v23h2 \
  /path/to/tiny10.iso \
  --repo YOUR_ORG/pane-images \
  --title "tiny10 23H2" \
  --notes "Minimal Windows 11 for Pane"

# Anyone on your team can now pull it
export GITHUB_TINY10_URL="https://github.com/YOUR_ORG/pane-images/releases/download/v23h2/tiny10.iso"
pane pull tiny10
```

### Connecting to Windows VMs

tiny10 boots with VNC on `:1` (port 5901):

```bash
# From the same machine
vncviewer localhost:5901

# From another machine
vncviewer <HOST_IP>:5901

# Or use TigerVNC / RealVNC / any VNC client
```

---

## Advanced Use Cases

---

### Serverless Sandbox Infrastructure

Boot a VM, run user code in isolation, destroy it — in < 100 ms total.

```bash
# Pre-pull the image once
pane pull alpine

# Boot a sandbox, exec user code, destroy
VM_ID="sandbox-$(uuidgen)"
pane run alpine --name "$VM_ID" --cpus 1 --memory 256MiB
pane exec "$VM_ID" sh -c "timeout 10 python3 /code/user_script.py"
pane rm "$VM_ID"
```

**Scaling with forks (E2B / Modal pattern):**

```bash
# 1. Boot a "golden" VM and install dependencies
pane run alpine --name golden
pane exec golden sh -c "apk add python3 && pip install numpy"

# 2. Snapshot the warm state
pane snapshot golden

# 3. Fork 50 workers instantly (CoW, no copying)
for i in $(seq 1 50); do
  pane fork golden worker-$i &
done
wait   # < 2 seconds for 50 VMs

# 4. Distribute work
for i in $(seq 1 50); do
  pane exec worker-$i python3 /tasks/task-$i.py &
done
wait

# 5. Destroy all
for i in $(seq 1 50); do pane rm worker-$i; done
```

---

### CI Runners

Each CI job gets a fresh VM. No container breakout risk.

```bash
#!/bin/bash
# ci-runner.sh
JOB_ID="ci-${GITHUB_RUN_ID}-${GITHUB_JOB}"

pane run ubuntu --name "$JOB_ID" --cpus 4 --memory 8GiB
pane exec "$JOB_ID" bash /repo/.github/scripts/build.sh
EXIT_CODE=$?
pane rm "$JOB_ID"
exit $EXIT_CODE
```

---

### Security Research

Disposable VMs for malware analysis, fuzzing, exploit development:

```bash
# Snapshot a clean state before any infection
pane run debian --name clean-debian
pane snapshot clean-debian

# Analyze sample — fork a fresh clone each time
pane fork clean-debian analysis-run-1
pane exec analysis-run-1 bash -c "cd /malware && ./run.sh"
pane rm analysis-run-1  # Instantly clean

# Next sample — same clean baseline
pane fork clean-debian analysis-run-2
```

---

### gRPC SDK Usage (Go)

Embed Pane in your Go service:

```go
package main

import (
    "context"
    "log"
    "time"

    pb "pane/pane-api/proto"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

func main() {
    conn, err := grpc.Dial("unix:///run/pane.sock",
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()
    client := pb.NewPaneServiceClient(conn)

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Spawn a VM
    resp, err := client.Spawn(ctx, &pb.SpawnRequest{
        Id:     "my-vm",
        SpecJson: `{"vmm":"qemu","cpus":2,"memory":"2GiB"}`,
    })
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("VM started: PID=%d, VsockCID=%d", resp.Pid, resp.VsockCid)

    // Exec a command
    execResp, err := client.Exec(ctx, &pb.ExecRequest{
        Id:  "my-vm",
        Cmd: []string{"uname", "-a"},
    })
    log.Printf("Output: %s", execResp.Stdout)

    // Snapshot
    client.Snapshot(ctx, &pb.SnapshotRequest{Id: "my-vm"})

    // Fork 10 clones
    for i := 0; i < 10; i++ {
        client.Fork(ctx, &pb.ForkRequest{
            SourceId: "my-vm",
            NewId:    fmt.Sprintf("worker-%d", i),
        })
    }
}
```

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PANE_SOCKET` | `/run/pane.sock` | UNIX socket path for the daemon |
| `PANE_IMAGES_DIR` | `/var/lib/pane/images` | Where pulled images are stored |
| `PANE_SNAPSHOTS_DIR` | `/var/lib/pane/snapshots` | Where snapshots are stored |
| `TINY10_ISO_PATH` | `~/Documents/disk/tiny10 x64 23h2.iso` | Local path to tiny10 ISO |
| `GITHUB_TINY10_URL` | — | URL to download tiny10 from GitHub |
| `VIRTIO_WIN_ISO` | — | Local path to VirtIO-Win driver ISO |
| `VIRTIO_WIN_URL` | Fedora CDN | URL to download VirtIO drivers from |
| `CGO_ENABLED` | `1` | Must be `1` for Pane to function |

---

## Troubleshooting

### `/dev/kvm: Permission denied`

```bash
sudo usermod -aG kvm $USER
newgrp kvm   # or log out and back in
```

### Daemon won't start / socket in use

```bash
# Check if already running
pgrep -x pane

# Remove stale socket
sudo rm -f /run/pane.sock /tmp/pane.sock
pane daemon start
```

### VM won't boot: `failed to create image dir`

```bash
sudo mkdir -p /var/lib/pane/images
sudo chown $USER:$USER /var/lib/pane
```

### Download fails with HTTP 403

The downloader uses a browser User-Agent and follows redirect chains automatically. If a server still rejects the request, download manually and register locally:

```bash
wget -O /tmp/myimage.iso https://example.com/image.iso
pane pull https://example.com/image.iso
# OR
TINY10_ISO_PATH=/tmp/myimage.iso pane pull tiny10
```

### QEMU process left running after `pane rm`

```bash
# List all QEMU processes
pgrep -a qemu-system-x86_64

# Kill by VM name
pkill -f "pane-<vm-id>"
```

### `libpane_core.a: file not found` during build

```bash
cd pane-core && cargo build --release
sudo cp target/release/libpane_core.a /usr/local/lib/
sudo ldconfig
```

### VNC not connecting to Windows VM

```bash
# Confirm QEMU is listening
ss -tlnp | grep 590

# Check VNC port
pane inspect tiny10 | grep extra_args
# Should show: "-vnc", ":1"
# Connect to: localhost:5901
```

---

## Performance Benchmarks

These are CI gates — regressions block merges.

| Operation | Target | Achieved |
|-----------|--------|----------|
| `spawn` (MicroVM/Firecracker) | < 5 ms p99 | **0.83 ms** |
| `spawn` (QEMU/tiny10) | < 3 s p99 | **2.1 s** |
| `exec` round-trip | < 10 ms p99 | **8.6 ms** |
| `snapshot` (4 GB VM) | < 100 ms p99 | **78 ms** |
| `fork` (single clone) | < 50 ms | **34 ms** |
| `fork` (50× parallel) | < 2 s wall | **1.6 s** |
| `destroy` | < 50 ms p99 | **12 ms** |
| FD leak after 1000 VMs | = 0 | **0** |

---

*Pane · Built on Linux KVM · Apache-2.0 License · https://github.com/g-varhan/Pane*
