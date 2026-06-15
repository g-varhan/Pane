# ADR 0005 — eBPF Network Isolation via TC

| Field       | Value                |
|-------------|----------------------|
| Status      | Accepted             |
| Date        | 2024-01-01           |
| Deciders    | pane-vmm maintainers |
| Supersedes  | —                    |

## Context

Multi-tenant VM deployments require network isolation between VMs that share
the same host TAP bridge. Three approaches were evaluated:

| Approach | Granularity | Kernel support | Runtime overhead |
|----------|-------------|----------------|-----------------|
| iptables / nftables | Per-IP rule | All kernels | Moderate (ruleset traversal) |
| network namespaces | Per-namespace | All kernels | Setup complexity, no cross-ns |
| **eBPF TC classifier** | Per-packet, programmable | ≥ 4.1 (stable ≥ 5.8) | Near-zero (JIT compiled) |

## Decision

Pane attaches a **TC (Traffic Control) eBPF classifier** to each VM's TAP
interface to enforce group-based network isolation.

Key design decisions:

1. **Group map**: VMs are assigned to numeric group IDs. An eBPF `HashMap`
   (`IFACE_GROUP_MAP`) keyed by interface index stores each VM's group.
2. **Classifier logic**: On `TC_ACT_SHOT` (drop) if source and destination
   groups differ, `TC_ACT_OK` otherwise. Cross-group traffic is silently
   dropped at the kernel fast-path.
3. **aya crate**: The eBPF program is compiled with `aya-ebpf` and loaded at
   runtime via the `aya` userspace library, eliminating a separate `bpftool`
   dependency.
4. **Attach point**: `tc` `clsact` qdisc, `BPF_TC_INGRESS` hook on each TAP.

## Consequences

**Positive**
- Isolation enforced at kernel fast-path — no userspace packet copy.
- Group membership changes are O(1) map updates, no iptables rule re-install.
- Composable: future policies (rate-limiting, logging) can be added as
  additional eBPF tail-call programs.

**Negative**
- Requires kernel ≥ 5.8 for stable eBPF TC support (same requirement as
  cgroups v2, so no additional constraint).
- eBPF programs must be compiled with a separate LLVM/clang target; the
  pre-compiled object is checked into `pane-core/src/bpf/`.
- `CAP_NET_ADMIN` or `CAP_BPF` required to load TC programs.

## References

- `pane-core/src/network.rs` — `init_network_ebpf()`, `register_vm_network_group()`
- `pane-core/src/bpf/` — eBPF source and compiled object
- [aya-rs.dev](https://aya-rs.dev) — Rust eBPF framework
- [kernel.org: tc-bpf](https://www.kernel.org/doc/html/latest/networking/filter.html)
