
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## $(date +%Y-%m-%d) - [Arbitrary File Deletion / Path Traversal]
**Vulnerability:** In `pane-api/panespec/images.go`, functions like `RemoveImage`, `InspectImage`, and `PullImage` failed to validate if the parsed/cleaned image name was `.` or `..` before joining it with the base images directory. An attacker could pass these values to execute operations (like `os.RemoveAll`) on the base directory itself.
**Learning:** Sanitizing inputs by replacing slashes (`/`) and colons (`:`) is insufficient against `.` and `..` because they do not contain those characters. Furthermore, when `.` or `..` is joined with a base path via `filepath.Join`, it resolves to the base directory or its parent, escaping the intended sub-directory scope for the operation.
**Prevention:** When constructing file paths using user-supplied identifiers intended to be sub-directory names, always explicitly validate and reject empty strings, `.`, and `..` immediately before calling `filepath.Join`.
