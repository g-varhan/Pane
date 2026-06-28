
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## 2024-06-28 - [Path Traversal in Destructive Operations]
**Vulnerability:** The `RemoveImage` and `InspectImage` functions performed `os.RemoveAll` and file path construction using an image name that wasn't properly validated. It only sanitized `/` and `:`, leaving `.` and `..` alone, which allowed an attacker to delete the entire `images` parent directory by passing `..` as the image name.
**Learning:** Path sandboxing (e.g. replacing `/` or joining to a base dir) is insufficient when user-supplied identifiers are used for destructive operations like `os.RemoveAll`. When a path includes `..`, it escapes the base directory, causing the `os.RemoveAll` to delete the base directory instead of an intended subdirectory.
**Prevention:** Always use strict input validation to explicitly reject empty strings, `.`, and `..` from user-supplied names before using them in file paths for destructive operations.
