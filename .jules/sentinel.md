## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## 2026-07-07 - [Path Traversal in Image Handling]
**Vulnerability:** Insufficient validation of user-supplied image names used in file path construction allowed path traversal and potentially arbitrary file deletion (via `os.RemoveAll`) or exposure in `pane-api/panespec/images.go`. Inputs like `.` and `..` were not rejected before `filepath.Join`.
**Learning:** Path sandboxing (like `secureJoin`) is insufficient when untrusted inputs evaluate directly to the base directory itself (e.g. `.` or `..`).
**Prevention:** Always implement strict input validation to explicitly reject empty strings, `.`, and `..` before using them to construct paths for destructive operations or file system access.
