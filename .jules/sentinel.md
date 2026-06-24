
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.
## 2026-06-24 - [Environment Variable Corruption]
**Vulnerability:** In `pane-api/server/handler.go`, `os.Setenv` and `os.Unsetenv` were used to temporarily set environment variables for child processes (VMs) without tracking the host's original environment.
**Learning:** Blindly unsetting environment variables after temporarily modifying them can corrupt the host's environment if those variables existed prior to the modification.
**Prevention:** Track original values using `os.LookupEnv` before modifying, and restore them properly using `os.Setenv` (if they existed) or `os.Unsetenv` (if they did not) to ensure the host environment remains untainted.
