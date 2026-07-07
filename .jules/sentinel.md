
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## 2024-05-20 - [Path Traversal in Image Management]
**Vulnerability:** The image management functions (`PullImage`, `RemoveImage`, and `InspectImage`) in `pane-api/panespec/images.go` constructed paths using `filepath.Join` without validating the user-provided image name against path traversal characters like `.` and `..` or empty strings. `secureJoin` is insufficient when the untrusted part evaluates strictly to the base directory itself.
**Learning:** Functions performing destructive file operations (like `os.RemoveAll`) or accessing sensitive configuration must validate input immediately. Even after standard sanitization, combinations like empty strings (often from stripped prefixes like `docker://`) or `.` / `..` can lead to unexpected critical failures.
**Prevention:** Always implement strict input validation explicitly rejecting empty strings, `.`, and `..` when user-supplied identifiers (such as image names) are used in file paths, especially before path construction or traversal operations.
