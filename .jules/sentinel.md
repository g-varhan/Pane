
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.
## 2026-06-23 - Environment Variable Leakage
**Vulnerability:** The Go application modifies the global process environment variables before calling a Rust FFI function and restores them afterward. If an environment variable was set prior, the unset operation destroys it.
**Learning:** Modifying the global process environment variables can lead to corrupting the host process environment variable state.
**Prevention:** Always track and restore original values using os.LookupEnv before calling os.Setenv and os.Unsetenv.
