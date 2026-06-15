# Logging Conventions

This document describes how Pane components emit log output and how to
interpret and configure it.

---

## Log Framework

| Layer | Language | Logger |
|-------|----------|--------|
| `pane-vmm` | C | `fprintf(stderr, ...)` — structured prefix tags |
| `pane-core` | Rust | [`tracing`](https://docs.rs/tracing) crate |
| `pane-api` / `pane-cli` | Go | standard `log` package (structured via `slog` if available) |

---

## `pane-core` (Rust) — tracing levels

Pane uses the `tracing` ecosystem. Log levels follow standard semantics:

| Level | When to use | Example |
|-------|-------------|---------|
| `ERROR` | Unrecoverable failure; action cannot proceed | cgroup directory could not be removed |
| `WARN` | Degraded operation; action proceeds but with caveats | vsock socket path fell back to `/tmp/pane` |
| `INFO` | Normal lifecycle events | VM started, VM destroyed |
| `DEBUG` | Internal state transitions, config values | Firecracker API response body |
| `TRACE` | Per-packet / per-syscall detail | QMP raw JSON frames |

### Setting log level

Set `RUST_LOG` before running `pane-api`:

```bash
# INFO and above (production default)
RUST_LOG=pane_core=info,pane_api=info ./pane-api

# Full debug output from pane-core only
RUST_LOG=pane_core=debug ./pane-api

# Trace everything (very noisy)
RUST_LOG=trace ./pane-api
```

`RUST_LOG` supports the full
[`EnvFilter`](https://docs.rs/tracing-subscriber/latest/tracing_subscriber/filter/struct.EnvFilter.html)
directive syntax.

---

## `pane-vmm` (C) — stderr prefix tags

The C VMM layer writes to `stderr` with a consistent prefix:

```
[PANE-VMM INFO]  ...
[PANE-VMM WARN]  ...
[PANE-VMM ERROR] ...
```

When running as a child of `pane-core`, `pane-vmm` stderr is inherited by the
parent process and interleaved with Rust tracing output. To separate them,
redirect `pane-api` stderr to a file and filter by prefix:

```bash
./pane-api 2>pane.log
grep "\[PANE-VMM" pane.log
```

---

## `pane-api` / `pane-cli` (Go)

Go components use the standard `log` package with an `INFO:` / `WARN:` /
`ERROR:` prefix. In future releases this will be migrated to `slog` for
structured JSON output.

Set `LOG_LEVEL=debug` environment variable to enable verbose output from Go
components:

```bash
LOG_LEVEL=debug ./pane-api
```

---

## Sensitive Data in Logs

The following values are **never logged** at any level:

- Guest memory contents.
- VM rootfs paths that may contain user data.
- vsock connection payloads (exec stdin/stdout).

The following values appear only at `DEBUG` or `TRACE`:

- Firecracker API request/response bodies (may contain kernel paths and boot args).
- QMP raw JSON frames (may contain snapshot file paths).

---

## Structured Fields (Rust `tracing`)

All `pane-core` spans and events include consistent structured fields:

| Field | Type | Description |
|-------|------|-------------|
| `vm_id` | `&str` | The VM's string identifier |
| `backend` | `"firecracker"` \| `"native"` | Which backend handles the VM |
| `state` | `"spawning"` \| `"running"` \| `"frozen"` \| `"dead"` | Current VM state |
| `pid` | `u32` | Backend process PID (when known) |
| `cgroup_path` | `&Path` | Cgroup directory path |

Example log line (JSON subscriber):
```json
{"timestamp":"2024-01-15T10:23:01Z","level":"INFO","target":"pane_core::vm",
 "vm_id":"sandbox-42","backend":"firecracker","state":"running",
 "message":"VM started successfully"}
```

---

## Log Aggregation

For production deployments, pipe `pane-api` output to a log aggregator
(e.g., `journald`, `fluentd`, `Vector`). When using `systemd`:

```ini
[Service]
ExecStart=/usr/local/bin/pane-api
StandardError=journal
Environment=RUST_LOG=pane_core=info,pane_api=info
```

Logs are then queryable with:
```bash
journalctl -u pane-api -f
journalctl -u pane-api --output=json | jq 'select(.vm_id=="sandbox-42")'
```
