
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.
## 2025-02-26 - [Path Traversal in Destructive File Operations]
**Vulnerability:** Path sanitization functions that replace specific characters (like `/` and `:`) do not protect against traversal payloads like `.` and `..` or empty strings when combined with functions like `os.RemoveAll`.
**Learning:** When building paths for destructive actions (e.g. `os.RemoveAll(filepath.Join(baseDir, name))`), input validation must explicitly reject empty strings, `.`, and `..`. Path sanitization logic like `strings.ReplaceAll` is insufficient on its own.
**Prevention:** Implement strict input validation to explicitly reject empty strings, `.`, and `..` before path construction and usage in file system operations.

## 2025-02-26 - [Environment Variable State Corruption]
**Vulnerability:** Unconditionally using `os.Unsetenv` after temporarily setting an environment variable corrupts the host process if the variable was already present.
**Learning:** Modifying process environment variables affects the entire host application. Unsetting them after use will wipe out any pre-existing environment variables with the same keys, potentially leading to widespread failures.
**Prevention:** Always track the original values of environment variables using `os.LookupEnv` before calling `os.Setenv`, and restore them to their original values after use instead of blindly calling `os.Unsetenv`.
