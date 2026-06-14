Pane v0.2 — Unified CLI, Dynamic Config & Container Compatibility (Revised)

Implementation Prompt for Antigravity

0. Project Context (read first, carry forward in every phase)

Pane is a lightweight, embeddable KVM-based VM lifecycle control plane for Linux — think

"SQLite for VM management." It is not a platform competing with VMware/Proxmox/AWS; the

goal is to be the dependency other infrastructure products (Proxmox, E2B, CI runners, AI

sandbox startups) build on. Never propose features framed as "compete with X platform."

Architecture (three languages, clear boundaries):


pane-vmm (C) — raw KVM ioctls, io_uring, guest memory, virtio. ~15 core functions.

No business logic. No config parsing.

pane-core (Rust) — VM state machines, cgroups v2, eBPF (TC via aya, never XDP),

snapshot/fork logic, tokio async. No unwrap()/expect() in lib code. Everyunsafe block requires a // SAFETY: comment.

pane-cli / pane-api (Go) — CLI, gRPC server, OCI/registry interactions, YAML/JSON

config parsing, CGo FFI into Rust.

Solo-dev fallback: C core + Go everything; add Rust pieces incrementally.

Five primitives already in scope for v0.1 (do not regress these):


spawn <5ms MicroVM / <3s QEMU

exec <10ms vsock round-trip

snapshot <100ms for 4GB RAM

fork CoW clone <50ms/VM, 50 forks in <2s

destroy <50ms, zero leaks

Hard technical rules — never hallucinate, always verify against real sources:


KVM ioctl constants → read /usr/include/linux/kvm.h or kernel source directly.

io_uring → verify SQE field names against /usr/include/liburing/io_uring.h;

never skip io_uring_cqe_seen().

cgroups → v2 only (memory.max, cpu.max, pids.max, cgroup.kill). Never v1.

eBPF → TC via aya, not XDP.

Firecracker → Unix socket REST API; track_dirty_pages: true required for snapshots.

vsock → Host CID=2, guest CID per-VM, port 1024 = exec, port 1025 = logs,

static-linked guest agent.

CoW fork → cp --reflink=always; requires btrfs/xfs (NOT ext4) — fail fast if reflink

unsupported; RAM file mmap'd MAP_PRIVATE.

Process spawning for VMMs → always execve with an argv array. Never system()

or popen(). This is non-negotiable for security once config files / extra_args are

user-controlled.

Naming/paths conventions to preserve:


TAP devices: pane-{vm_id}-tap0

State/data root: /var/lib/pane/

Image cache: /var/lib/pane/images/<name>/<version>/

1. Goal of This Development Cycle (v0.2)

Four feature areas, designed to be additive to the existing five-primitive core, not a

rewrite of it:


Unified CLI — collapse pane-api (daemon) and pane (VM ops CLI) into one binary

with subcommand dispatch (pane daemon ..., pane run, etc.), supporting both

"talk to running daemon" and "standalone/embedded" modes.

Dynamic QEMU configuration — replace hardcoded qemu.c argv construction with a

config struct + composable argv builder, driven entirely by inline CLI flags orimage metadata (no per-use config files required). The schema (panespec) defines

validation and merge logic, but users primarily interact via flags like --cpus 4 --memory 2GiB.

Image metadata + pane pull — when pulling an image (curated manifest or

OCI-converted), write metadata.json + panespec.json alongside the disk image sopane run <image> auto-configures without further setup.

pane-ctr: Docker/OCI image → bootable Pane image — a separate conversion

component (Go) that turns an OCI image into a disk image + kernel + init, consumable

by the same run/fork primitives. No new C code required in pane-vmm.

Unifying abstraction: introduce a single "Pane image" concept (disk image + kernel +

metadata.json + panespec.json + optional OCI metadata) that pull, images, run, andfork all operate on identically, regardless of source. Users interact via:


pane run <image> — use defaults from image metadata

pane run --cpus 4 <image> — override one field via flag

pane run -f config.yaml <image> — load a saved config (optional, for teams/CI)

2. Phase Plan

Execute phases in order. Each phase should land as an independently mergeable,

independently testable unit. Do not start a phase's "build" tasks until its "design"

tasks are reviewed.

Phase 0 — panespec Schema & Shared Config Contract

Why first: every other phase (image metadata, CLI flags, QEMU builder, pane-ctr output)

needs to agree on one config shape. Getting this right first avoids rework.

Design todo:


[ ] Define panespec v1 schema (canonical form: Go struct; serializable to YAML/JSON,

same shape) covering:vmm: backend selector (qemu | firecracker)

cpus, memory (with unit suffixes, e.g. 512MiB, 2GiB)

disk: { path, size, format } (raw | qcow2)

image: reference string (pane://..., docker://..., oci://..., local path)

network: mode (none | bridge | nat) + optional bridge name

drivers: { virtio_net: bool, virtio_blk: bool, virtio_rng: bool }

kernel / cmdline: optional, for direct-kernel-boot images

extra_args: []string raw passthrough (QEMU-only escape hatch)

[ ] Document precedence rules: CLI flags > file (pane.yaml) > image metadata

(panespec.json) > hardcoded defaults.

[ ] Write the "default profile" — a panespec that reproduces today's hardcodedqemu.c behavior exactly (this becomes the regression baseline for Phase 2).

[ ] Decide where panespec parsing/validation lives (Go), and define the plain C struct

that pane-vmm actually receives (no YAML/JSON ever touches C).

[ ] Map each CLI flag (--cpus, --memory, --disk, --iso, --kernel, etc.)

to its corresponding panespec field.

Build todo:


[ ] Go struct + YAML/JSON (de)serialization for panespec.

[ ] Validation function: clear errors for invalid/missing fields, unit parsing

(512MiB → bytes), path existence checks for disk.path/kernel.

[ ] Merge function: takes (CLI flags as partial panespec) + (file panespec) +

(image metadata panespec) + (hardcoded defaults), applies precedence, returns final.

[ ] pane config init — scaffold a commented example pane.yaml (for teams/CI who

want saved configs; optional).

[ ] pane config validate <file> — run validation on a hand-edited YAML/JSON, print

human-readable errors.

[ ] Define the C-side struct (pane_vmm_config_t) mirroring the validated subset

needed by pane-vmm.

Acceptance criteria:


[ ] A pane.yaml round-trips through parse → validate → C struct without loss for all

fields above.

[ ] Merge logic correctly implements precedence: CLI flag --cpus 4 overrides image

metadata cpus: 2, which overrides the hardcoded default cpus: 1.

[ ] The "default profile" produces a struct identical to today's hardcoded values.

Phase 1 — Unified CLI Skeleton

Goal: single pane binary, subcommand dispatch, daemon vs. client vs. standalone

modes clearly separated. No new VM functionality yet — this is plumbing.

Design todo:


[ ] Decide CLI framework (e.g. cobra if Go) and overall command tree (see §3 below).

[ ] Define the client transport contract: Unix socket + gRPC to pane daemon, with a

"standalone" fallback that links pane-core directly for single-host/dev use

(important: Pane is embeddable — the CLI must not assume a daemon is mandatory).

[ ] Define daemon lifecycle commands and where PID/socket files live under/var/lib/pane/.

[ ] Wire the CLI flag definitions for pane run (see Phase 0's flag → field mapping).

Build todo:


[ ] Scaffold pane binary with subcommand routing; merge existing pane-api

entrypoint into pane daemon start.

[ ] Implement pane daemon start|stop|status|reload.

[ ] Implement transport abstraction (DaemonClient interface) with two

implementations: grpcClient (talks to running daemon) and embeddedClient

(direct in-process calls to pane-core via existing FFI).

[ ] Port existing five-primitive commands (run/exec/snapshot/fork/rm or

equivalent current names) onto the new transport abstraction — behavior must be

unchanged, this is a plumbing refactor only.

[ ] Define pane run CLI flags (from Phase 0 mapping):--cpus N              → panespec.cpus--memory SIZE         → panespec.memory (parse units: 512MiB, 2GiB, etc.)--disk SIZE           → panespec.disk.size--iso PATH            → panespec.disk.path (ISO-based boot override)--kernel PATH         → panespec.kernel--cmdline "..."       → panespec.cmdline--virtio-net          → panespec.drivers.virtio_net = true--no-virtio-net       → panespec.drivers.virtio_net = false--virtio-blk          → panespec.drivers.virtio_blk = true--no-virtio-blk       → panespec.drivers.virtio_blk = false--gui / --display GTK → panespec.extra_args: ["-display", "gtk"]-f, --config FILE     → load panespec from YAML/JSON file (Phase 0 merge)--dry-run             → build the panespec, print the QEMU argv, do not exec--name NAME           → VM name for tracking-- <args...>          → raw QEMU args (appended last, after extra_args)

[ ] pane version — report CLI, daemon (if reachable), and core library versions.

Acceptance criteria:


[ ] All existing five-primitive operations work identically through the new binary,

both against a running daemon and in standalone mode.

[ ] No regression in spawn/exec/snapshot/fork/destroy latency targets.

[ ] pane run --dry-run ubuntu-22.04 (before Phase 3) prints help/error indicating

image not found.

Phase 2 — Dynamic QEMU Configuration (qemu.c Refactor + CLI merge)

Goal: qemu.c accepts a pane_vmm_config_t struct and produces a QEMU argv array via

composable builder functions. CLI flags, image metadata, and config files merge correctly

to produce that struct. No hardcoded paths/args remain except as the documented default

profile from Phase 0.

Design todo:


[ ] Finalize pane_vmm_config_t fields (mirrors Phase 0 schema subset):typedef struct {    uint32_t vcpus;    uint64_t memory_bytes;    const char *disk_path;    const char *disk_format;     /* "qcow2" | "raw" */    bool virtio_net;    bool virtio_blk;    bool virtio_rng;    const char *net_bridge;      /* may be NULL */    const char *kernel_path;     /* may be NULL for full-disk boot */    const char *cmdline;         /* may be NULL */    const char **extra_args;     /* NULL-terminated argv-style passthrough */} pane_vmm_config_t;

[ ] Define the vmm_backend interface so QEMU and (future) Firecracker builders share

a dispatch point without coupling.

[ ] Enumerate the QEMU builder functions and their ordering:qemu_args_add_machine, qemu_args_add_cpu_mem, qemu_args_add_disk,qemu_args_add_net (gated on virtio_net), qemu_args_add_rng (gated onvirtio_rng), qemu_args_add_kernel (gated on kernel_path != NULL),qemu_args_add_extra (always last — raw passthrough).

[ ] Merge logic in Go (Phase 1 / Phase 0): CLI flags + config file + image metadata

→ validated panespec → convert to C struct.

[ ] Plan the test matrix: minimal config, full virtio, kernel-direct-boot, Windows +

virtio-win, Docker-converted rootfs, config with extra_args.

Build todo:


[ ] Implement qemu_argv_t (growable argv buffer) with qemu_argv_new,qemu_argv_push, qemu_argv_free, qemu_argv_to_string (for dry-run/debug).

[ ] Implement each qemu_args_add_* builder as an independent, unit-testable function.

[ ] Wire execve-based spawn to consume the built argv (confirm no system()/popen()

remain anywhere in the spawn path).

[ ] Go-side merge logic (Phase 1 pane run command):Parse CLI flags into a partial panespec.

Load panespec from image metadata (populated by Phase 3 pane pull).

Load panespec from file (if --config provided).

Apply merge precedence: CLI > file > image metadata > defaults.

Validate the result.

Serialize to C struct via FFI.

[ ] Add pane run --dry-run — runs the full merge + builder pipeline and prints the

resulting argv without exec'ing.

[ ] Golden-file tests: fixed pane_vmm_config_t inputs → expected argv arrays, covering:minimal config (defaults only)

full virtio (net + blk + rng)

kernel-direct-boot

Windows guest + virtio-win driver ISO attached

config with extra_args passthrough

the "default profile" from Phase 0 (must match pre-refactor output byte-for-byte)

Acceptance criteria:


[ ] Zero hardcoded QEMU args remain outside the default-profile definition.

[ ] All golden-file tests pass.

[ ] pane run --dry-run --cpus 4 --memory 2GiB ubuntu-22.04 (Phase 3 prerequisite) shows

correct argv with merged config (once Phase 3 populates image metadata).

[ ] pane run --dry-run -f custom.yaml ubuntu-22.04 correctly applies file precedence

over image metadata.

[ ] CLI flags correctly override image metadata: --cpus 4 in CLI beats cpus: 2 from

image.

Phase 3 — Image Metadata + Curated Manifest (pane pull, scoped)

Goal: when pulling an image, write metadata.json + panespec.json alongside the

disk image. pane run <image> auto-configures using that metadata. Enable pane pull to

work with both curated-manifest images and OCI conversions (Phase 4). This is where the

magic happens: pane run ubuntu-22.04 with zero flags just works.

Design todo:


[ ] Define the image metadata layout:/var/lib/pane/images/ubuntu-22.04/v1.0/├── disk.qcow2                          # actual disk image├── vmlinuz (optional)                  # guest kernel (for direct-boot)├── metadata.json                       # ← identifies image, VMM backend, drivers└── panespec.json                       # ← default panespec (cpus, memory, etc.)

[ ] Define metadata.json schema (minimal, read-only):{  "name": "ubuntu-22.04",  "version": "v1.0",  "vmm": "qemu",  "source": "pane://ubuntu-22.04",  "kernel_path": "/var/lib/pane/images/ubuntu-22.04/v1.0/vmlinuz",  "drivers_required": ["virtio-net", "virtio-blk"]}

[ ] Define panespec.json (defaults, user can override with CLI flags or custom yaml):{  "vmm": "qemu",  "cpus": 2,  "memory": "512MiB",  "disk": {    "path": "/var/lib/pane/images/ubuntu-22.04/v1.0/disk.qcow2",    "format": "qcow2"  },  "drivers": {    "virtio_net": true,    "virtio_blk": true,    "virtio_rng": false  }}

[ ] Define the curated image manifest (JSON/YAML, hosted on GitHub — no custom

registry server):{  "images": [    {      "name": "ubuntu-22.04",      "version": "v1.0",      "url": "https://cloud-images.ubuntu.com/releases/jammy/release/ubuntu-22.04-server-cloudimg-amd64.img",      "checksum_sha256": "...",      "vmm": "qemu",      "kernel_url": "https://...",      "drivers_required": ["virtio-net", "virtio-blk"],      "panespec_defaults": { "cpus": 2, "memory": "512MiB", "disk": {...} }    }  ]}

[ ] Decide image naming convention in cache: name:version or name/version; pick

one and stick with it.

[ ] Define pane pull <ref> scheme dispatch:pane://<name>[:version] → fetch from curated manifest, download image + kernel,

write metadata.json + panespec.json

docker://<image> / oci://<image> → delegate to pane-ctr (Phase 4), capture

output into metadata + panespec.json

https://...iso → direct HTTP fetch, register with minimal metadata

local path → symlink or copy, register with minimal metadata

[ ] Scope driver bundling correctly: most Linux images already have virtio drivers

built-in (cloud-init images from Ubuntu, Debian, Fedora). Only bundlevirtio-win.iso for Windows entries (to test the bundling code path).

Build todo:


[ ] Manifest fetch + parse (GitHub raw-content URL; HTTPS + simple JSON parse).

[ ] pane pull pane://<name>[:version] — download image + kernel, verify checksum,

write to cache, write metadata.json + panespec.json.

[ ] pane pull for docker:///oci:// refs — delegate to pane-ctr pull (Phase 4),

capture the output disk image + init + OCI sidecar, write them alongside

metadata.json + panespec.json (with OCI-specific annotations in metadata).

[ ] Modify pane run <image> command to discover and load panespec.json from the

image directory (if image name resolves to a cached image).

[ ] pane images — list cached/registered images, showing source + version + disk

size (source-agnostic: manifest-based and OCI-converted images appear identically).

[ ] pane rmi <image> — remove cached image + metadata.

[ ] pane image inspect <image> — show resolved final panespec for the image (after

merging defaults, before CLI overrides).

[ ] Seed the initial curated manifest with: Ubuntu 22.04/24.04 cloud image, Debian 12,

Alpine 3.18+, Fedora cloud, and one Windows entry (with virtio-win.iso reference)

as the driver bundling test case.

[ ] Error handling: image not found → clear message pointing user to pane pull.

Acceptance criteria:


[ ] pane pull pane://ubuntu-22.04 downloads the image, writes metadata.json +

panespec.json.

[ ] pane run ubuntu-22.04 discovers the cached image, loads panespec.json, and

spawns a VM with zero CLI flags needed.

[ ] pane run --cpus 4 ubuntu-22.04 overrides the cpus field from panespec.json.

[ ] pane images lists the image with its source and size.

[ ] Re-running pane pull pane://ubuntu-22.04 is a no-op if the image is cached

(checksum match).

Phase 4 — pane-ctr: OCI/Docker Image → Bootable Pane Image

Goal: a standalone Go component that converts an OCI image reference into a Pane

image (disk image + kernel reference + init + OCI metadata), consumable by run/fork

exactly like a Phase 3 image. No changes to pane-vmm required — this phase produces

inputs, not new VMM code.

Design todo:


[ ] Confirm registry client library (go-containerregistry recommended) — handles

auth, multi-arch manifests, layer pulls.

[ ] Define the conversion pipeline stages explicitly:

Fetch OCI manifest + layers via registry client (same as docker pull).

Flatten layers into rootfs — extract + overlay layer tarballs, correctly handle

OCI whiteout files (.wh.*) for deletions.

Rootfs → disk image — mke2fs -d <rootfs> (e2fsprogs) with sized headroom

(e.g., 50% buffer); document first-boot resize2fs/growpart step in the init.

Guest kernel — reuse the generic guest kernel(s) already used for the five

primitives (do not build a per-image kernel pipeline).

Custom init (PID 1) — a small static binary that mounts /proc, /sys, /dev,

starts the existing vsock guest agent (ports 1024/1025), reads the OCI config

sidecar, and exec's the container's process.

OCI config sidecar — extracted entrypoint/cmd/env/workdir/user, written as JSON

alongside disk image for init to read.

Port mapping metadata — extract EXPOSE directives; user can override with

explicit --publish at run time.

Cache key — OCI image digest → converted disk image (reuse if pulled again).

[ ] Define the output layout (same as Phase 3 image dir):


/var/lib/pane/images/nginx/latest/

├── disk.raw or .qcow2

├── vmlinuz (symlink or copy of generic guest kernel)

├── metadata.json                       # with "source": "docker://nginx:latest"

├── panespec.json                       # defaults from image: cpus, memory, etc.

├── init (static binary, PID 1)

└── oci-config.json                     # entrypoint, cmd, env, workdir, user

[ ] Guest-side init contract (small static PID-1 binary, Go or C):

mounts /proc, /sys, /dev

starts vsock guest agent (ports 1024/1025, reuse existing daemon code)

reads oci-config.json

setups UID/GID (if user present)

chdir to workdir (if present)

sets env vars

exec the entrypoint (or cmd fallback) as PID 1

if entrypoint/cmd fail, spawn a fallback shell for debugging

[ ] Caching: image digest → converted disk image path. Cache dir must live on

btrfs/xfs (same constraint as existing fork primitive) so pane fork on

container-derived images works via reflink. Fail fast with clear error if cache dir

is on ext4.

[ ] Port mapping: extract EXPOSE directives from OCI config. For now, scoped to

explicit --publish HOST:GUEST at run time (no automatic port forwarding in v1 —

reduce surface area).

[ ] Rootfs resource estimation: scan the image size and recommend sane defaults forcpus, memory, disk in the output panespec.json.

Build todo:


[ ] pane-ctr pull <docker-ref> (Go standalone tool, or integrated into pane pull):

fetch OCI manifest + layers

extract + flatten into tmpfs/temp directory

mke2fs -d <rootfs> to build disk image

find or link the guest kernel

build + compile the static init binary

extract OCI config, write oci-config.json

write metadata.json + panespec.json to the output image dir

register image in /var/lib/pane/images/

[ ] Static init binary (Go, statically linked, ~5-10MB; or minimal C if size is critical):

mounts + vsock agent startup

oci-config.json parsing

uid/gid + env setup

exec container entrypoint

[ ] Wire pane pull docker://<ref> → internally calls pane-ctr pull → outputs image

to standard Pane image dir with metadata.json + panespec.json, so pane run treats

it identically to curated-manifest images.

[ ] Conversion cache keyed on OCI image digest; check for re-pulls of the same digest.

Verify cache dir supports reflink (btrfs/xfs); fail with clear error on ext4.

[ ] pane run --publish 8080:80 nginx:latest — discover ports from OCI metadata,

set up host-side DNAT to guest virtio-net port.

Acceptance criteria:


[ ] pane pull docker://nginx:latest downloads the image, converts it, writes disk

image + init + metadata.json + panespec.json.

[ ] pane run nginx:latest --publish 8080:80 boots the image and serves nginx on

host port 8080.

[ ] pane fork nginx:latest --count 5 --prefix nginx-prod creates 5 CoW clones via

existing reflink-based fork (no special-casing needed).

[ ] Re-pulling the same image digest hits the conversion cache (no re-conversion).

[ ] If conversion cache is on ext4, error message is clear and actionable: "Conversion

cache requires btrfs or xfs. Configure with PANE_IMAGE_CACHE=/path/on/btrfs".

3. CLI Command Map (target end-state after all phases)

# Daemon control

pane daemon start [--config /etc/pane/daemon.yaml] [--socket /run/pane.sock]

pane daemon stop

pane daemon status

pane daemon reload


# VM lifecycle (all work identically regardless of image source)

pane run [FLAGS] <image>

  --name NAME                              # VM name

  --cpus N                                 # override panespec default

  --memory SIZE (512MiB, 2GiB, etc.)       # override panespec default

  --disk SIZE                              # override panespec default

  --iso PATH                               # use ISO directly (override disk.path)

  --kernel PATH                            # override or set panespec.kernel

  --cmdline "..."                          # override panespec.cmdline

  --virtio-net / --no-virtio-net           # override panespec.drivers.virtio_net

  --virtio-blk / --no-virtio-blk           # override panespec.drivers.virtio_blk

  --gui / --display MODE                   # add GUI (gtk, sdl, qxl, etc.) to extra_args

  -f, --config FILE                        # load panespec from YAML/JSON file

  --dry-run                                # print QEMU argv, don't execute

  -- <extra-args...>                       # raw QEMU args (appended last)


pane ps [--all]

pane inspect <vm>

pane logs <vm>

pane exec <vm> -- <cmd...>

pane snapshot <vm> [--tag NAME]

pane fork <vm> [--count N] [--prefix NAME]

pane stop <vm>

pane rm <vm>


# Image management (all sources look identical to user)

pane pull <ref>

  # pane://ubuntu-22.04       (curated manifest)

  # docker://nginx:latest     (OCI conversion)

  # https://...iso            (direct fetch)

  # /path/to/local.iso        (local file)


pane images [--all]

pane rmi <image>

pane image inspect <image>


# Configuration (optional; most users skip this)

pane config init                           # scaffold a pane.yaml template

pane config validate <file>                # validate YAML/JSON against schema


# Metadata

pane version

4. User Experience Examples (the goal)

Example 1: Most Common — Just Run an Image

pane pull pane://ubuntu-22.04

pane run ubuntu-22.04

# → defaults from metadata: 2 CPUs, 512MiB RAM, 10GB disk, virtio-net/blk

Example 2: Quick Override

pane run --cpus 4 --memory 2GiB ubuntu-22.04

# → 4 CPUs, 2GiB RAM, disk + network defaults from metadata

Example 3: Docker Workload

pane pull docker://nginx:latest

pane run --publish 8080:80 nginx:latest

# → nginx booting in a microVM, served on host:8080

Example 4: Saved Config (Teams/CI)

# config.yaml

cpus: 4

memory: 2GiB

disk: 50GiB


pane run -f config.yaml ubuntu-22.04

# → merges file + image metadata, file wins on conflicts

Example 5: One-Off Custom ISO

pane run --iso /custom.iso --kernel /custom/vmlinuz --cmdline "root=/dev/vda1"

# → image metadata provides virtio drivers, user provides kernel/ISO

Example 6: Dry Run (Debug)

pane run --dry-run --cpus 4 ubuntu-22.04

# → prints the merged panespec + QEMU argv, exits

5. Cross-Cutting Constraints (apply in every phase)

[ ] No system()/popen() anywhere in process-spawn paths — execve + argv only.

[ ] No new code paths touch ext4 for reflink/fork operations — fail fast on

unsupported filesystems with a clear, actionable error message.

[ ] cgroups v2 only; no v1 fallback code.

[ ] vsock CID/port conventions (host CID=2, ports 1024/1025) reused as-is bypane-ctr's init — do not introduce a second agent protocol.

[ ] Every new unsafe block in Rust gets a // SAFETY: comment.

[ ] Every new ioctl/io_uring usage is checked against the real headers

(/usr/include/linux/kvm.h, /usr/include/liburing/io_uring.h) before being

written — do not recall constants from memory.

[ ] panespec remains backend-agnostic at the top level; QEMU-specific behavior is

confined to extra_args and the QEMU builder, never leaking into shared schema

fields.

[ ] Merge precedence is always CLI > file > image metadata > defaults, enforced

consistently across all commands that load a panespec.

6. Suggested Execution Order & Dependencies

Phase 0 (panespec schema) ──┬──> Phase 1 (unified CLI skeleton + Phase 0 merge logic)

                             └──> Phase 2 (qemu.c refactor + CLI flag → panespec wiring)

                                        │

Phase 1 + Phase 2 ──────────────────────┴──> Phase 3 (image metadata + pane pull)

                                                     │

Phase 3 ─────────────────────────────────────────> Phase 4 (pane-ctr / OCI conversion)

Parallelization:


Phase 0 design must complete before Phase 1 + Phase 2 build tasks start.

Phase 1 and Phase 2 can proceed in parallel once Phase 0 schema is settled.

Phase 1's CLI flag definitions and Phase 2's builder functions are independent;

they meet in the middle via the merge logic.

Phase 3 depends on both Phase 1 (CLI) and Phase 2 (config plumbing) to wire up

the metadata loading.

Phase 4 (pane-ctr) depends on Phase 3's image registration path but introduces

no new pane-vmm code.

7. Success Metrics

By the end of Phase 4, a user should be able to:


# One-shot bootstrap for a standard workload

pane pull pane://ubuntu-22.04

pane run ubuntu-22.04


# or

pane pull docker://your-app:latest

pane run your-app:latest


# with optional overrides for one-off experiments

pane run --cpus 8 --memory 4GiB your-app:latest


# and no config files, no scripts, no ceremony.

That's the bar: zero config files, CLI flags + image metadata = complete system.