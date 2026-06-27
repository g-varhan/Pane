
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## $(date +%Y-%m-%d) - [Arbitrary File Deletion / Path Traversal]
**Vulnerability:** Path traversal in `pane-api/panespec/images.go` file deletion logic allowed arbitrary directory deletion. `filepath.Join("/var/lib/pane/images", name)` would resolve to `/var/lib/pane` if `name` was `..`, resulting in the deletion of the entire pane directory when passed to `os.RemoveAll`.
**Learning:** For file operations that involve deleting, inspecting, or mutating files, input provided by users (such as an image name) must be explicitly sanitized to reject empty strings `""`, current directory `.`, and parent directory `..`.
**Prevention:** Add explicit input validation such as `if name == "" || name == "." || name == ".." { return err }` before utilizing user-provided paths in file system operations.
