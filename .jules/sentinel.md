
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## $(date +%Y-%m-%d) - [Path Traversal in Image Deletion]
**Vulnerability:** The `RemoveImage` and `InspectImage` functions in `pane-api/panespec/images.go` constructed paths for destructive file operations using unfiltered user input `name`. While `/` and `:` were filtered, inputs like `.` and `..` were not, leading to path traversal where `filepath.Join(dir, "..")` resolves to the base directory. This allowed `os.RemoveAll` to delete the entire `images` base directory if an attacker passed `..` as an image name.
**Learning:** Path sandboxing or string replacement for directory separators (`/`) is insufficient when dealing with inputs like `.` or `..` which `filepath.Join` natively evaluates. When constructing paths that will be used for destructive operations, user input intended as a single directory name must be explicitly validated against `.` and `..` directly.
**Prevention:** Always validate user-supplied filenames to explicitly reject empty strings `""`, `.` and `..` before performing any string manipulation or path joining.
