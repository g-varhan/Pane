
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## 2026-06-26 - [Arbitrary Directory Deletion]
**Vulnerability:** `RemoveImage` in `pane-api/panespec/images.go` used `filepath.Join` with an unvalidated image name. Empty string, `.`, or `..` could be used to resolve to the base image directory or higher, leading to arbitrary directory deletion via `os.RemoveAll`.
**Learning:** When using untrusted identifiers in destructive operations like `os.RemoveAll` on constructed paths, relying on `filepath.Join` or path sandboxing is insufficient. Input values like empty strings, `.`, or `..` will bypass naive traversal checks because they resolve cleanly relative to the base, resulting in deletion of the parent directory itself.
**Prevention:** Always strictly validate user-supplied identifiers (such as image or container names) explicitly rejecting empty strings, `.`, and `..` before combining them into paths destined for file system modification functions.
