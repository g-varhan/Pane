# VM Lifecycle State Machine

This document describes the states a Pane VM can occupy, the transitions
between them, and the invariants enforced at each boundary.

---

## State Diagram

```
                     ┌──────────────────────────────────────────┐
                     │              NEW VM ID                   │
                     └──────────────────┬───────────────────────┘
                                        │ new_firecracker()
                                        │ new_native()
                                        ▼
                              ┌─────────────────┐
                              │    Spawning     │◄──────────────────┐
                              └────────┬────────┘                   │
                                       │ spawn()                    │
                                       │ configure_*(...)           │ fork_firecracker()
                                       │ start()                    │ (enters as Frozen)
                                       ▼                            │
                              ┌─────────────────┐                   │
                         ┌───►│    Running      │                   │
                         │    └────────┬────────┘                   │
                    resume()           │ freeze()                   │
                         │             ▼                            │
                         │    ┌─────────────────┐◄─────────────────┘
                         └────┤     Frozen      │
                              └────────┬────────┘
                                       │
                        destroy() ◄────┴────► destroy()
                        (from any state)
                                       │
                                       ▼
                              ┌─────────────────┐
                              │      Dead       │
                              └─────────────────┘
```

---

## States

### `Spawning`

The VM has been allocated in memory but is not yet executing guest code.

**Allowed operations:**
- `spawn()` — launches the backend process (Firecracker) and attaches cgroup.
- `configure_machine()` — sets vCPU count and memory.
- `configure_boot_source()` — sets kernel image and boot arguments.
- `configure_drive()` — attaches rootfs block device.
- `configure_vsock()` — configures the guest vsock CID and UDS path.
- `configure_network_interface()` — attaches TAP interface.
- `load_snapshot()` — loads a Firecracker snapshot (pre-boot alternative to configure_*).
- `start()` → transitions to **Running**.
- `destroy()` → transitions to **Dead** (abort before boot).

**Invariants:**
- No guest code is running.
- cgroup may already be attached (after `spawn()`).

---

### `Running`

The VM is actively executing guest code.

**Allowed operations:**
- `exec()` — runs a command inside the guest via vsock.
- `freeze()` → transitions to **Frozen**.
- `destroy()` → transitions to **Dead**.
- `assume_running(id)` — reconstructs a `Vm<Running>` from a persisted VM ID
  (bypasses typestate; caller guarantees the VM is actually running).

**Invariants:**
- Backend process is alive and accepting vsock connections.
- cgroup is attached and active.

---

### `Frozen`

The VM's guest vCPUs are paused; memory is intact.

**Allowed operations:**
- `resume()` → transitions to **Running**.
- `create_snapshot()` — writes a Firecracker snapshot to disk (snapshot file + memory file).
- `patch_drive()` — hot-patches a block device path (used in fork flows).
- `configure_vsock()` — updates vsock config on a fork before resume.
- `configure_network_interface()` — updates network on a fork before resume.
- `destroy()` → transitions to **Dead**.
- `assume_frozen(id)` — reconstructs a `Vm<Frozen>` from a persisted VM ID.

**Invariants:**
- Backend process is alive but guest CPUs are halted.
- Memory file is valid if a snapshot was previously created.

---

### `Dead`

The VM has been terminated; all associated resources have been released.

**Allowed operations:** None. The `Vm<Dead>` value is a tombstone; it can be
dropped. No further operations are defined on `Vm<Dead>`.

**Cleanup performed on entry:**
1. Backend process killed (`SIGKILL` or QMP `quit`).
2. QMP socket file removed (if present).
3. cgroup directory destroyed (`cgroup.kill` + `rmdir`).

---

## Fork Flow

`Vm::fork_firecracker()` is a convenience constructor that produces a
`Vm<Frozen>` directly, bypassing the `Spawning` phase:

```
Spawn new Firecracker process (Spawning)
  └─► load_snapshot(parent_snap, parent_mem)
        └─► return Vm<Frozen>   ← ready to patch drives/vsock and resume
```

This is the primary mechanism for container forking from a pre-warmed
snapshot. See `docs/adr/0003-dual-backend.md` for rationale.

---

## `assume_running` / `assume_frozen` — Safety Contract

These constructors exist to re-hydrate VM handles after a `pane-api` restart,
when the VM is actually running but no in-process `Vm<Running>` handle exists.

**Caller must guarantee:**
- The process identified by the VM ID is actually in the claimed state.
- The cgroup directory and socket files exist.

If these guarantees are violated, subsequent operations on the reconstructed
handle will return errors (socket not found, process not responding, etc.).
