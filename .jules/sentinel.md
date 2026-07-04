
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## 2026-07-04 - [Arbitrary File Deletion via Path Traversal]
**Vulnerability:** The `RemoveImage` and `InspectImage` functions in `pane-api/panespec/images.go` constructed file system paths using `filepath.Join(dir, name)` where `name` was derived directly from untrusted input without validation against empty strings or path traversal sequences (`.` and `..`).
**Learning:** For destructive filesystem operations (like `os.RemoveAll`), standard path sanitization (e.g. `filepath.Clean`) or sandboxing (e.g. `secureJoin`) is insufficient when dealing with root-level traversal identifiers like `.` or `..`, because they resolve to the base directory itself. An input of `""` evaluated to the base dir in some combinations, and `..` evaluated to the parent.
**Prevention:** Always implement strict input validation to explicitly reject empty strings, `.`, and `..` (and any variations that sanitize to them) *before* constructing file paths using untrusted identifiers.
