
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.
## 2026-06-30 - [Destructive Path Traversal in Image Deletion]
**Vulnerability:** Untrusted image names were sanitized for slashes and colons but not for relative path identifiers (`.` and `..`) or empty strings. This allowed an attacker to delete the entire base image directory when `os.RemoveAll` was called.
**Learning:** Sandboxing path names for safe resource deletion requires strict validation. Sanitizing slashes is insufficient if empty strings, `.`, or `..` are passed in, as these resolve to the base directory or its parent directory during `filepath.Join`.
**Prevention:** Implement strict input validation to explicitly reject empty strings and path traversal sequences (`.` and `..`) prior to path construction when dealing with destructive operations like `os.RemoveAll`.
