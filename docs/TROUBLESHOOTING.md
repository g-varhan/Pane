# Troubleshooting Guide

Common issues encountered when running Pane and their resolutions.

---

## 1. `No writeable cgroup v2 directory found`

**Symptom**: `pane-api` exits immediately with:
```
ERROR pane_core::resources: No writeable cgroup v2 directory found
```

**Cause**: The process has no write access to any cgroup v2 subtree.

**Resolution**:

| Scenario | Fix |
|----------|-----|
| Running as root on a standard Linux system | Check that `/sys/fs/cgroup` is a cgroup v2 unified hierarchy: `stat -f /sys/fs/cgroup` should show `Type: cgroup2fs`. If not, your kernel or distro mounts a hybrid/v1 hierarchy — see your distro docs to enable v2. |
| Running as non-root without systemd user session | Ensure `systemd --user` is running: `systemctl --user status`. If not active, enable lingering: `loginctl enable-linger $USER`. |
| Running inside a container | The container must have the cgroup v2 subtree delegated. For Docker: `docker run --cgroup-parent=/user.slice --cgroupns=host ...`. For Podman: `podman run --cgroup-manager=cgroupfs ...`. |
| CI environment | Pre-create `/sys/fs/cgroup/pane` and `chown` it to the CI user, or run the test suite as root. |

---

## 2. Firecracker Process Exits Immediately

**Symptom**: `VM spawn failed` or `Firecracker API unreachable` shortly after `start()`.

**Resolution**:

1. Check that `firecracker` is installed and on `$PATH`:
   ```bash
   which firecracker && firecracker --version
   ```
2. Check kernel support for KVM:
   ```bash
   ls /dev/kvm && ls -l /dev/kvm
   ```
   The calling user must have read/write access to `/dev/kvm`. Add to the `kvm` group:
   ```bash
   sudo usermod -aG kvm $USER
   # log out and back in
   ```
3. Inspect Firecracker's own stderr (interleaved in `pane-api` output) for its
   error message — common causes: missing kernel image path, bad rootfs format,
   insufficient memory.

---

## 3. `QMP socket connect failed`

**Symptom**: Operations on QEMU-backed VMs return:
```
PaneError::Socket("QMP socket connect failed: ...")
```

**Cause**: The QMP socket file does not exist or QEMU is not listening.

**Resolution**:

1. Verify QEMU is running and the PID in `/run/pane/qemu-<id>.pid` is alive:
   ```bash
   cat /run/pane/qemu-<id>.pid
   kill -0 <pid>
   ```
2. Check that the QMP socket file exists:
   ```bash
   ls -la /run/pane/qmp-<id>.sock
   # or fallback:
   ls -la /tmp/pane/qmp-<id>.sock
   ```
3. If both the process and socket exist but the connection still fails, QEMU
   may not have finished its QMP handshake. Add a short retry loop in your
   caller, or check `RUST_LOG=pane_core=debug` for the raw error.

---

## 4. `exec()` Hangs or Returns EOF Immediately

**Symptom**: `Vm<Running>::exec()` connects but the stream returns EOF before
the command runs.

**Resolution**:

1. Verify the Pane guest agent is running inside the VM:
   ```bash
   # from inside the guest:
   ps aux | grep pane-agent
   ```
2. Check the vsock CID matches what was configured with `configure_vsock()`.
   The default CID is `3`; if multiple VMs share a host, each needs a unique CID.
3. Verify the vsock UDS path on the host:
   ```bash
   ls -la /run/pane/fc-vsock-<id>.sock
   ```
4. On Firecracker backends, ensure the vsock device was configured before
   calling `start()`. Configuring vsock on an already-running VM has no effect.

---

## 5. `cow_clone_rootfs` returns `ENOTSUP`

**Symptom**: Fork operations fail with an OS error `95` (ENOTSUP / EOPNOTSUPP).

**Cause**: The filesystem hosting the rootfs directory does not support
`cp --reflink=always` (e.g., ext4, tmpfs, NFS).

**Resolution**:

Option A — Use btrfs or XFS for the rootfs store:
```bash
# Example: create a btrfs loopback for the rootfs store
dd if=/dev/zero of=/var/pane-store.img bs=1G count=20
mkfs.btrfs /var/pane-store.img
mount -o loop /var/pane-store.img /var/lib/pane/rootfs/
```

Option B — Implement a slow-path fallback in the orchestration layer:
```rust
match pane_core::cow_clone_rootfs(src, dst) {
    Err(PaneError::Io(e)) if e.raw_os_error() == Some(libc::ENOTSUP) => {
        // Fall back to full copy
        std::fs::copy(src, dst)?;
    }
    other => other?,
}
```

---

## 6. VM Destroy Leaves Stale cgroup Directory

**Symptom**: After a crash, `/sys/fs/cgroup/pane/<vm-id>/` persists and blocks
new VM creation with the same ID.

**Resolution**:

```bash
# Ensure no processes are in the cgroup first
cat /sys/fs/cgroup/pane/<vm-id>/cgroup.procs

# Kill all tasks if any remain
echo 1 > /sys/fs/cgroup/pane/<vm-id>/cgroup.kill

# Remove the directory
rmdir /sys/fs/cgroup/pane/<vm-id>/
```

---

## 7. eBPF TC Program Not Loaded / Network Isolation Not Active

**Symptom**: VMs in different groups can communicate with each other.

**Resolution**:

1. Check that `init_network_ebpf()` was called at `pane-api` startup.
2. Verify the TC qdisc is attached to the TAP interface:
   ```bash
   tc qdisc show dev tap0
   # Should show: qdisc clsact ...
   tc filter show dev tap0 ingress
   # Should show: filter protocol all ... bpf ...
   ```
3. Check `CAP_NET_ADMIN` is available to the `pane-api` process:
   ```bash
   capsh --print | grep net_admin
   ```
4. If the eBPF program failed to load, check `dmesg` for verifier errors:
   ```bash
   dmesg | grep -i bpf | tail -20
   ```

---

## 8. `pane-api` Fails to Start: `CGO_ENABLED` / FFI Link Error

**Symptom**: `pane-api` binary fails to start with a missing symbol or library error.

**Resolution**:

Pane's Go API uses CGo to call `pane-core` and `pane-vmm` via FFI. Both
static libraries must be built before the Go binary:

```bash
# 1. Build C VMM library
make -C pane-vmm

# 2. Build Rust core library
cd pane-core && cargo build --release && cd ..

# 3. Build Go API (CGo will link against the static libs)
cd pane-api && CGO_ENABLED=1 go build -o pane-api . && cd ..
```

Verify the libraries exist:
```bash
ls -lh pane-vmm/libpane_vmm.a
ls -lh pane-core/target/release/libpane_core.a
```

---

## Getting More Help

- File a bug: [https://github.com/g-varhan/Pane/issues](https://github.com/g-varhan/Pane/issues)
- Enable verbose logging: `RUST_LOG=pane_core=debug,pane_api=debug`
- Check the [internals docs](internals/) for architecture details
