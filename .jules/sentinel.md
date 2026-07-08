
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## 2024-07-08 - [Path Traversal in Image Handling]
**Vulnerability:** `RemoveImage`, `InspectImage`, and `PullImage` functions used user-provided image names to construct file paths without validating that the name did not contain path traversal sequences (`.` or `..`) or was empty. This allowed arbitrary file deletion and out-of-bounds reads.
**Learning:** Functions that accept user input to construct paths for destructive operations must strictly validate the input before construction. `filepath.Join` and similar sanitization methods (like replacing `/` with `-`) are insufficient because inputs like `.` and `..` will still traverse the directory structure. Furthermore, prefix-stripping logic (e.g., in `PullImage`) can result in empty strings if only the prefix is provided, bypassing subsequent checks if not explicitly validated.
**Prevention:** Always validate identifiers used in path construction against a strict allowlist or explicit denylist (rejecting empty strings, `.`, and `..`) immediately before building the path.
