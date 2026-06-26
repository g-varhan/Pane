
## 2026-06-26 - [Path Traversal in Arbitrary File Deletion]
**Vulnerability:** Untrusted image identifiers containing `.` or `..` were passed directly to `filepath.Join` and evaluated before `os.RemoveAll`. This is dangerous because an image name like `.` bypasses string replacement sanitization, leading to the deletion of the entire image directory base path `/var/lib/pane/images/`.
**Learning:** For destructive operations (`os.RemoveAll`), sandboxing with `filepath.Join` or `filepath.Clean` is not enough to protect against special directories like `.` and `..` because they resolve to the target directory or its parent directory. Replacing slashes `/` doesn't block `.` or `..` or empty strings either.
**Prevention:** Always implement explicit input validation checks (`if name == "" || name == "." || name == ".."`) on user-controlled inputs used in path construction, especially for functions performing `os.RemoveAll` or file system interactions, rejecting these identifiers before performing any path operations.

## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.
