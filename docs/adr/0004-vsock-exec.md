# ADR 0004 — vsock + Unix Domain Socket for Guest Exec

| Field       | Value                |
|-------------|----------------------|
| Status      | Accepted             |
| Date        | 2024-01-01           |
| Deciders    | pane-vmm maintainers |
| Supersedes  | —                    |

## Context

Pane must execute commands inside guest VMs and stream their output back to
the host without requiring a network interface in every VM. Options considered:

| Mechanism | Setup complexity | Overhead | Guest changes needed |
|-----------|-----------------|----------|----------------------|
| SSH over virtio-net | High (DHCP/IP setup) | TLS + TCP | Full SSH daemon |
| virtio-serial (console) | Low | Very low | minicom / custom agent |
| **vsock + UDS proxy** | Low | Low | Pane guest agent only |
| gVisor runsc exec API | N/A (only gVisor) | Low | N/A |

## Decision

Pane uses **AF_VSOCK** (virtio socket) as the transport between host and guest,
proxied through a Unix Domain Socket (UDS) on the host side.

- **Host side**: `Vm<Running>::exec()` connects to the UDS at
  `/run/pane/fc-vsock-<id>.sock` (or `/tmp/pane/...` fallback) and speaks
  the Pane exec protocol over it.
- **Guest side**: a lightweight agent (`pane-core/src/agent.c`) listens on
  vsock port 52 and executes commands, streaming stdout/stderr back.
- **Firecracker**: the vsock UDS is configured via the Firecracker API at
  `configure_vsock()`.
- **Native/QEMU**: the vsock device is set up by `pane-vmm` directly.

The exec protocol is a simple length-prefixed JSON framing:
`[4-byte LE length][JSON ExecRequest]` → streaming `ExecChunk` responses.

## Consequences

**Positive**
- No guest networking required; exec works in fully isolated VMs.
- vsock is guest-ID addressed, avoiding IP allocation entirely.
- UDS on the host is file-system controlled, easy to permission-gate.

**Negative**
- Requires the Pane agent to be baked into every guest rootfs.
- vsock CID management is manual (default CID=3, configurable via
  `configure_vsock()`); collisions must be avoided by the caller.

## References

- `pane-core/src/exec.rs` — `exec_in_guest()`, `ExecStream`, `ExecChunk`
- `pane-core/src/agent.c` — guest-side vsock agent
- `pane-core/src/vm.rs` — `get_vsock_socket_path()`, `Vm<Running>::exec()`
- [Linux vsock man page](https://man7.org/linux/man-pages/man7/vsock.7.html)
