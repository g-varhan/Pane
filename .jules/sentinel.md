
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## $(date +%Y-%m-%d) - [Path Traversal / Arbitrary File Deletion in Image Manager]
**Vulnerability:** The image management logic (`RemoveImage`, `InspectImage`, `PullImage`) in `pane-api/panespec/images.go` did not validate image names against `.` or `..` or empty strings. A call like `RemoveImage("")` would evaluate to `os.RemoveAll("/var/lib/pane/images")`, deleting all images.
**Learning:** `filepath.Join(dir, name)` where `name` can evaluate to empty string returns the `dir` itself. Thus, operations like `os.RemoveAll(filepath.Join(dir, ""))` delete the root storage directory.
**Prevention:** Always validate user-supplied identifiers (like image names) that will be appended to base directories to reject empty strings, `.`, and `..` before passing them to `filepath.Join()`.
