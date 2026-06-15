# ADR 0001 — Use cgroups v2 for VM Resource Isolation

| Field       | Value                |
|-------------|----------------------|
| Status      | Accepted             |
| Date        | 2024-01-01           |
| Deciders    | pane-vmm maintainers |
| Supersedes  | —                    |

## Context

Pane needs a way to impose per-VM resource limits (CPU, memory, process count) that:

- Works without a privileged daemon process.
- Supports both root and unprivileged (user-systemd) deployments.
- Is available on all modern Linux distributions shipping kernel ≥ 5.8.

Three options were considered:

| Option | Pros | Cons |
|--------|------|------|
| **cgroups v1** | Broad kernel support | Legacy; not available as unified hierarchy; will be removed in future kernels |
| **cgroups v2** | Unified hierarchy, better semantics, supported by systemd user-session slices | Requires kernel ≥ 4.5 (full feature set ≥ 5.8) |
| **rlimits (setrlimit)** | No root needed | Per-process only; cannot govern the entire VM subtree |

## Decision

Pane uses **cgroups v2 exclusively** via the unified hierarchy at `/sys/fs/cgroup`.

The `CgroupManager` in `pane-core/src/resources.rs` discovers the writable cgroup root at runtime using a three-level probe:

1. `/sys/fs/cgroup/pane` — root/system-wide path (privileged daemons).
2. `/sys/fs/cgroup/user.slice/user-<uid>.slice/user@<uid>.service/pane` — user-session path for unprivileged use via systemd.
3. `/proc/self/cgroup` climb — generic fallback by walking up from the current process's own cgroup until a writable `pane/` subtree is found.

Enabled controllers: `+cpu +memory +pids`.

## Consequences

**Positive**
- Single code path for all deployment modes (system, user session, containers).
- `cgroup.kill` support (kernel ≥ 5.14) allows instantaneous VM teardown without iterating tasks.
- Works inside rootless-container environments that delegate a v2 subtree.

**Negative**
- Kernels older than 5.8 are unsupported (acceptable for our target: Ubuntu 22.04+, Fedora 37+, Arch).
- Unprivileged mode depends on systemd user sessions being active; bare-metal setups without systemd require pre-created cgroup directories.

## References

- `pane-core/src/resources.rs` — `get_cgroup_base_path()`, `CgroupManager`
- [kernel.org: Control Group v2](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html)
