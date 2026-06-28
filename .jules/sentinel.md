
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.
## $(date +%Y-%m-%d) - [Path Traversal / Arbitrary Deletion in Image Management]
**Vulnerability:** In `pane-api/panespec/images.go`, the `RemoveImage`, `InspectImage`, and `PullImage` functions took unsanitized image names. While `/` and `:` were replaced with `-`, values like `""`, `.`, or `..` were not handled. For `RemoveImage`, this could lead to `os.RemoveAll` on the entire images directory or parent directories.
**Learning:** Path sandboxing alone is not always enough when user inputs (identifiers) are appended to a base path. An input of `.` or `..` bypasses path combination protections and directly resolves to base directories.
**Prevention:** Always explicitly validate that user-provided string inputs used in file paths (especially for destructive operations) are not empty strings, `.`, or `..` before performing path construction.
