
## 2026-07-07 - [Image Name Path Traversal]
**Vulnerability:** Functions handling user-supplied identifiers (e.g., image names) constructed file paths for operations like `os.RemoveAll` without explicit validation, allowing inputs like `.` and `..` to resolve to base directories, bypassing basic path sanitization like `filepath.Join` or `.ReplaceAll`.
**Learning:** Sandboxing paths using functions like `secureJoin` is insufficient and dangerous when handling direct destructive operations (`os.RemoveAll`) where inputs like `.` or `..` resolve back to the base directory itself. Strict input validation must be used to explicitly reject empty strings, `.`, and `..` *before* path construction.
**Prevention:** Always implement explicit validation routines (e.g., `validateImageName`) to rigidly verify identifiers for missing or traversal values before incorporating them into file operations.

## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.
