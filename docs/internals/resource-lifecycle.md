# Resource Lifecycle Map

This document traces how every system resource (processes, files, sockets,
cgroup directories) that Pane allocates is eventually freed.

---

## 1. VM Process (Firecracker)

| Event | Action | Code reference |
|-------|--------|----------------|
| `Vm<Spawning>::spawn()` | Spawns `firecracker` child process | `backends/firecracker.rs` |
| `Vm<Spawning>::start()` | Sends `InstanceStart` to Firecracker API | `backends/firecracker.rs` |
| `Vm<Running>::destroy()` | Calls `fc.kill()` → SIGKILL to child | `vm.rs::cleanup()` |
| `Vm<Frozen>::destroy()` | Same as above | `vm.rs::cleanup()` |
| `Vm<Spawning>::destroy()` | Same as above | `vm.rs::cleanup()` |
| Process drops (panic / early return) | `FirecrackerVm::drop()` sends SIGKILL | `backends/firecracker.rs` |

> **Leak risk**: If the `pane-api` process exits abnormally before calling
> `destroy()`, orphaned `firecracker` child processes remain. Operators should
> monitor for `firecracker` processes not listed in the gRPC `ListVMs`
> response and kill them manually, or use a process supervisor that sets the
> child's pgroup and kills the entire group on exit.

---

## 2. QMP / vsock Unix Domain Sockets

| Path | Created by | Freed by |
|------|-----------|---------|
| `/run/pane/qmp-<id>.sock` | QEMU on startup | `vm.rs::cleanup()` — sends `quit` via QMP, then `fs::remove_file` |
| `/tmp/pane/qmp-<id>.sock` | QEMU (fallback) | Same |
| `/run/pane/fc-vsock-<id>.sock` | Firecracker on startup | Firecracker removes on exit; also `fs::remove_file` in cleanup |
| `/tmp/pane/fc-vsock-<id>.sock` | Firecracker (fallback) | Same |

Socket paths use `/run/pane` when writable (checked at runtime via
`is_run_pane_writable()`), with `/tmp/pane` as a fallback for unprivileged
environments.

---

## 3. cgroup v2 Directories

| Path | Created by | Freed by |
|------|-----------|---------|
| `<base>/pane/<vm-id>/` | `CgroupManager::create()` | `CgroupManager::destroy()` |

Destruction sequence in `destroy()`:
1. Write `1` to `cgroup.kill` (kernel ≥ 5.14) — signals all tasks.
2. Retry `fs::remove_dir` up to 5 times with 50 ms backoff (kernel needs time
   to evacuate tasks after `cgroup.kill`).
3. Return error if directory still exists after retries.

> **Leak risk**: If `CgroupManager::destroy()` is never called (e.g., process
> crash), the cgroup directory persists. It is safe to manually remove with
> `rmdir /sys/fs/cgroup/pane/<vm-id>` after the VM process has exited.

---

## 4. QEMU PID Files

| Path | Created by | Freed by |
|------|-----------|---------|
| `/run/pane/qemu-<id>.pid` | `pane-vmm` C layer on QEMU spawn | `pane-vmm` on QEMU exit; not explicitly cleaned by Rust |

> **Note**: Stale PID files from crashed VMs are detected by `assume_running()`
> reading a PID that no longer exists. The caller should validate the PID via
> `/proc/<pid>/status` before trusting it.

---

## 5. Network TAP Interfaces & eBPF Maps

| Resource | Created by | Freed by |
|----------|-----------|---------|
| TAP interface (e.g., `tap0`) | `pane-vmm` or host setup | Must be freed by host setup script or `ip link del` |
| `IFACE_GROUP_MAP` entry | `register_vm_network_group()` | `unregister_vm_network_group()` |
| TC eBPF qdisc | `init_network_ebpf()` | `tc qdisc del` (not done automatically) |

> **Known gap**: TC eBPF qdiscs and TAP interfaces are not automatically
> cleaned up on VM destroy. The `uninstall.sh` script handles global teardown;
> per-VM cleanup is the responsibility of the orchestration layer above
> `pane-api`.

---

## 6. Exec Streams

| Resource | Created by | Freed by |
|----------|-----------|---------|
| `UnixStream` to vsock UDS | `exec_in_guest()` | Dropped when `ExecStream` is dropped |
| Guest process (inside VM) | agent on vsock command | Guest agent manages lifetime; killed on VM destroy |

`ExecStream` implements `AsyncRead`; when dropped, the underlying
`UnixStream` is closed, which signals EOF to the guest agent.

---

## Summary: Resources That Require Explicit Cleanup

| Resource | Auto-freed on drop? | Manual cleanup path |
|----------|--------------------|--------------------|
| Firecracker process | ✅ (`FirecrackerVm::drop`) | `Vm::destroy()` |
| QMP socket file | ❌ | `vm.rs::cleanup()` → `destroy()` |
| vsock UDS file | ❌ | `vm.rs::cleanup()` → `destroy()` |
| cgroup directory | ❌ | `CgroupManager::destroy()` |
| QEMU PID file | ❌ | `pane-vmm` (external) |
| TAP interface | ❌ | Host setup / `uninstall.sh` |
| eBPF TC qdisc | ❌ | `uninstall.sh` / `tc` command |
| Exec stream | ✅ (Drop on `ExecStream`) | — |
