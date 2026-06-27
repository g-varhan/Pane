
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## 2026-06-27 - [Arbitrary File Deletion / Path Traversal]
**Vulnerability:** The `RemoveImage` and `InspectImage` functions in `pane-api/panespec/images.go` accepted an unvalidated image name, concatenated it with a base directory using `filepath.Join`, and executed operations like `os.RemoveAll`. Passing inputs like `.` or `..` directly allowed arbitrary deletion of the images base directory.
**Learning:** Simple string replacements (e.g. replacing `/` and `:`) are insufficient sanitization against path traversal. When inputs like `.` or `..` are passed to `filepath.Join`, they resolve precisely to the base directory or its parent, completely bypassing intended directory boundaries.
**Prevention:** Strictly validate untrusted inputs before appending them to base directories, explicitly rejecting exact strings like `""`, `.`, and `..`.
