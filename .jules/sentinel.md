
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## 2026-06-24 - [Environment Variable Corruption]
**Vulnerability:** The `Spawn` RPC handler temporarily modified the Go process environment variables with `os.Setenv` to pass them to the CGO/Rust FFI layer, but blindly called `os.Unsetenv` on all keys afterward, effectively deleting any pre-existing environment variables on the host process that shared the same keys.
**Learning:** Temporarily modifying a global state like the process environment must correctly handle the distinction between "variable exists but is empty" and "variable doesn't exist", as well as restoring variables to their original state rather than just deleting them.
**Prevention:** Always track the original state of environment variables (using `os.LookupEnv` to check if they were set) before overriding them, and use a `defer` block to guarantee restoration or deletion according to the original state, even if intermediate operations panic.
