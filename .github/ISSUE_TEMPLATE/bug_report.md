---
name: Bug Report
about: Report a reproducible bug in Pane
title: "[Bug] "
labels: bug
assignees: ''
---

## Description

<!-- A clear and concise description of what the bug is. -->

## Steps to Reproduce

1. 
2. 
3. 

## Expected Behavior

<!-- What did you expect to happen? -->

## Actual Behavior

<!-- What actually happened? Include any error messages or log output. -->

## Environment

| Field | Value |
|-------|-------|
| Pane version / commit | |
| OS / distro | |
| Kernel version (`uname -r`) | |
| Backend (Firecracker / Native QEMU / Native KVM) | |
| Running as root? | yes / no |
| cgroup v2 available? (`stat -f /sys/fs/cgroup`) | |

## Logs

```
# Paste relevant output from RUST_LOG=pane_core=debug ./pane-api
```

## Additional Context

<!-- Anything else that might help (config files, screenshots, etc.) -->
