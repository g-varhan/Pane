# ADR 0002 — Typestate Pattern for VM Lifecycle

| Field       | Value                |
|-------------|----------------------|
| Status      | Accepted             |
| Date        | 2024-01-01           |
| Deciders    | pane-vmm maintainers |
| Supersedes  | —                    |

## Context

VM management APIs are inherently stateful. Calling `freeze()` on a VM that is
already frozen, or calling `exec()` on a VM that has not yet started, are
logic errors that should be caught at compile time, not at runtime.

Common approaches:

| Approach | Compile-time safety | Runtime overhead |
|----------|---------------------|-----------------|
| Runtime state enum + guards | None | Mutex + branch per call |
| Typestate pattern (generic marker) | Full | Zero — monomorphised away |
| Session types (linear types) | Full | Zero | Rust lacks native linear types |

## Decision

`pane-core` uses the **typestate pattern** via sealed marker structs and a
`PhantomData` field on `Vm<State>`.

```
Spawning ──spawn()──► [configured] ──start()──► Running ──freeze()──► Frozen
                                                    │                     │
                                               destroy()            resume() / destroy()
                                                    ▼                     ▼
                                                  Dead               Running / Dead
```

Each state (`Spawning`, `Running`, `Frozen`, `Dead`) is a zero-sized struct
implementing the sealed `VmState` trait. Methods that are valid only in a
particular state are implemented on `Vm<ThatState>` exclusively, so any
misuse is a compile error.

## Consequences

**Positive**
- Impossible to call `exec()` on a frozen VM (not implemented for `Vm<Frozen>`).
- Impossible to call `freeze()` on a spawning VM (not implemented for `Vm<Spawning>`).
- Zero runtime cost: the generic is erased at monomorphization.
- Self-documenting API: the return type of `start()` is `Vm<Running>`, making
  state transitions explicit in function signatures.

**Negative**
- Storing heterogeneous VMs (e.g., a mix of `Running` and `Frozen`) in a
  single collection requires type-erasing via an enum wrapper or `Box<dyn …>`.
  Pane's gRPC layer uses string IDs and `assume_running` / `assume_frozen`
  constructors to re-hydrate VMs from persistent state.
- `assume_running` and `assume_frozen` are inherently unsafe from a state
  perspective (they bypass typestate guarantees). They are explicitly named to
  signal that the caller takes responsibility.

## References

- `pane-core/src/vm.rs` — `Vm<State>`, state marker structs
- [Rust Design Patterns: Typestate](https://rust-unofficial.github.io/patterns/patterns/behavioural/typestate.html)
