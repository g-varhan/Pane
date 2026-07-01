
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## 2024-05-20 - [Path Traversal in Destructive Operations]
**Vulnerability:** Functions `RemoveImage` and `InspectImage` used `filepath.Join` with user-supplied names after simple string replacement of `/` and `:`. Empty strings, `.`, or `..` were not rejected, causing path resolution to point to the base images directory (`/var/lib/pane/images`) or its parent directory, leading to catastrophic recursive deletion via `os.RemoveAll`.
**Learning:** Simple string sanitization (like replacing forward slashes) is insufficient for path safety in destructive operations. The inputs `""`, `.`, and `..` must be explicitly blocked because `filepath.Join` processes them structurally, potentially escaping intended boundaries.
**Prevention:** Always implement strict validation (e.g., `validateImageName`) to explicitly reject empty strings, `.`, and `..` before combining user input with a base directory for filesystem operations.
