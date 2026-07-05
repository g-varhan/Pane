
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## 2026-07-05 - [Path Traversal in Image Management]
**Vulnerability:** `RemoveImage`, `InspectImage`, and `PullImage` in `pane-api/panespec/images.go` did not validate image names against empty strings or path traversal sequences (like `.` and `..`) prior to using them in file path construction. This could allow deletion of critical directories or unexpected file access.
**Learning:** Path sandboxing (e.g., `secureJoin`) is insufficient and dangerous for user-supplied identifiers used in file paths when inputs like `.` or `..` resolve to the base directory itself. Always explicitly reject empty strings, `.`, and `..` before path construction.
**Prevention:** Always utilize the `validateImageName()` function to strictly reject empty strings and path traversal sequences (`.` and `..`) before constructing paths for operations.
