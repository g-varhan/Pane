
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## 2026-07-01 - [Arbitrary Directory Deletion via Path Traversal in Image Names]
**Vulnerability:** The `RemoveImage` and `InspectImage` functions in `pane-api/panespec/images.go` constructed target filesystem paths by appending unsanitized user inputs directly into `filepath.Join()`. While `/` and `:` were replaced, this sandboxing was insufficient because inputs like `.` or `..` resolved directly to the base directory itself or its parent, bypassing the sandbox entirely and leading to arbitrary file deletion when passed to `os.RemoveAll()`.
**Learning:** Path sanitization that only replaces typical directory separators (like `/`) is insufficient for preventing path traversal when the input represents an entire path component itself (like `.` or `..`). Since `filepath.Join` cleans the result, joining `/base` with `..` resolves to `/`, and joining with `.` resolves to `/base`.
**Prevention:** Always use strict input validation to explicitly reject empty strings, `.`, and `..` *before* path construction when dealing with user-supplied identifiers intended to be single path components (especially for destructive operations).
