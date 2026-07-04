
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## $(date +%Y-%m-%d) - [Path Traversal in Base Image Operations]
**Vulnerability:** The image management functions (`PullImage`, `RemoveImage`, and `InspectImage`) in `pane-api/panespec/images.go` accepted the identifiers `.` and `..` or identifiers that evaluate to an empty string. While standard paths with slashes were mitigated by conversion to hyphens, passing an empty string or `.` to `os.RemoveAll(filepath.Join(dir, name))` would result in `os.RemoveAll("/var/lib/pane/images")`, wiping out the entire base images directory.
**Learning:** `filepath.Join` in combination with path manipulations like `strings.TrimPrefix` can result in empty strings. Sandboxing the result (e.g., using `secureJoin`) does not prevent inputs like `.` and `..` from resolving directly to the target base directory, allowing operations on the directory itself rather than its contents.
**Prevention:** When handling user-supplied identifiers intended to be leaf nodes within a specific directory, explicitly validate the input to reject empty strings, `.`, and `..` *before* constructing any file paths.
