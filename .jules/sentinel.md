
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## 2026-07-08 - [Path Traversal in Image Management]
**Vulnerability:** In `pane-api/panespec/images.go`, user-supplied identifiers (image names) used in file paths for operations like `PullImage`, `RemoveImage`, and `InspectImage` were not properly validated. Inputs like `.` or `..` could resolve to unintended directories, bypassing basic sanitization that only handled `/` and `:`.
**Learning:** Functions handling user-supplied identifiers intended for file path construction must strictly reject path traversal sequences (like `.` or `..`) and empty strings. Simple character replacement (e.g., converting `/` to `-`) is insufficient if sequences like `..` remain intact and are later joined with base paths, potentially leading to critical file deletion or exposure.
**Prevention:** Always implement strict input validation to explicitly reject empty strings and path traversal sequences (`.`, `..`) prior to path construction and file system operations.
