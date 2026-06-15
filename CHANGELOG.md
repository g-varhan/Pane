# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-06-15

### Added
- Unified Go CLI dispatching `pane daemon`, `pane run`, `pane exec`, `pane snapshot`, and `pane fork`.
- Dynamic VMM configurations for QEMU hypervisor launching.
- Registry pull conversions via `pane-ctr` turning OCI/Docker container images into bootable VM images.
- Comprehensive system, API, and developer documentation guides.

### Changed
- Refactored `pane-vmm` C backend to accept configuration structs.
- Remediated developer-specific hardcoded paths with portable env-var and relative folder discovery.
- Code formatted and structured across C, Rust, and Go libraries.

### Security
- Swapped VMM process launching from shell wrappers to strict `execve` argument vector execution.

## [0.1.0] - 2026-06-14

### Added
- Initial release featuring the core 5 hypervisor primitives: `spawn`, `exec`, `snapshot`, `fork`, and `destroy`.
- KVM lifecycle controls, memory mappings, and `io_uring` block storage integration.
- eBPF-based group TC network filtering.
