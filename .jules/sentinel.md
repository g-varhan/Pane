
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## 2026-06-30 - [Path Traversal in Destructive Operations via Base Directory Escape]
**Vulnerability:** File removal operations in `pane-api/panespec/images.go` used `filepath.Join(dir, name)` with user-supplied `name`. An input of `.` resolved to the directory itself, causing `os.RemoveAll` to unexpectedly delete the entire base image directory.
**Learning:** For destructive operations, path sandboxing (like resolving out traversals) is not enough if the resulting path legitimately resolves to the root container directory itself. Special inputs like `.` and `..` must be explicitly blocked because `filepath.Join` standardizes them, potentially targeting the base directory or its parent.
**Prevention:** Always implement strict input validation to explicitly reject empty strings `""`, `.`, and `..` before path construction when handling user-supplied identifiers intended to be subdirectories or files.
