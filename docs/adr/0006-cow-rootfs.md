# ADR 0006 — CoW Rootfs Cloning via Reflink

| Field       | Value                |
|-------------|----------------------|
| Status      | Accepted             |
| Date        | 2024-01-01           |
| Deciders    | pane-vmm maintainers |
| Supersedes  | —                    |

## Context

Pane's fork-based container model (Firecracker snapshot fork) requires
producing per-fork rootfs images efficiently. For 100+ forks from the same
base image, copying gigabytes of data is prohibitive.

Options:

| Strategy | Copy overhead | Requires FS feature | Fallback |
|----------|--------------|---------------------|----------|
| `cp` (full copy) | O(size) time + space | None | Always works |
| `cp --reflink=always` | O(metadata) | btrfs / XFS | Fails fast (`ENOTSUP`) |
| `qcow2` + overlay | O(writes only) | qemu-img | Requires QEMU format |
| overlayfs mounts | O(metadata) | Linux overlayfs | Requires mount capability |

## Decision

`pane-core` uses `cp --reflink=always` via `cow_clone_rootfs()` for instant
Copy-on-Write rootfs duplication on supported filesystems (btrfs, XFS).

On unsupported filesystems, `cow_clone_rootfs()` returns
`Err(PaneError::Io(ENOTSUP))` immediately — it **does not fall back** to a
slow full copy. The caller (gRPC fork handler) is responsible for either:
- Providing a CoW-capable filesystem for the rootfs store, **or**
- Implementing a fallback strategy (e.g., `cp` without `--reflink`).

This fast-fail design is intentional: silent fallback to O(size) copies would
degrade multi-fork performance without warning.

## Consequences

**Positive**
- Fork startup time is O(metadata) on btrfs/XFS — essentially instant.
- No additional userspace dependencies; `cp` is universally available.
- Explicit error on unsupported FS lets operators choose the right storage.

**Negative**
- Production deployments must provision btrfs or XFS for the rootfs store
  directory (documented in the Getting Started guide).
- `ext4` and `tmpfs` do not support reflinking; forks on those filesystems
  will error unless the caller implements a slow-path fallback.

## References

- `pane-core/src/vm.rs` — `cow_clone_rootfs()`
- `docs/getstarted.md` — filesystem setup recommendations
- [btrfs reflink documentation](https://btrfs.readthedocs.io/en/latest/Reflinks.html)
- [XFS reflink support](https://xfs.org/index.php/XFS_FAQ#Reflink)
