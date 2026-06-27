
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## 2026-06-27 - [Path Traversal in Destructive Operations via Empty/Dot Inputs]
**Vulnerability:** User-supplied identifiers used in file paths for destructive operations like `os.RemoveAll` (e.g. `RemoveImage`) or path construction (e.g. `InspectImage`, `PullImage`) bypass sandboxing if they are evaluated to empty string, `.`, or `..`. This is because `filepath.Join` evaluates `.` or empty string to the base directory, and `..` to the parent directory. Calling `os.RemoveAll(filepath.Join(base, "."))` will delete the base directory itself.
**Learning:** When handling user-supplied identifiers (e.g., image names) used in file paths for operations like `os.RemoveAll`, string replacement is insufficient and path sandboxing (e.g., `secureJoin`) does not prevent inputs like `.` or `..` from resolving to the base directory or escaping it.
**Prevention:** Always use strict input validation to explicitly reject empty strings, `.`, and `..` before using them in path construction for destructive operations.
