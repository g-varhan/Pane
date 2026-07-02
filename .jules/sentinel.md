
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## 2025-02-23 - [Path Traversal in Image Deletion and Inspection]
**Vulnerability:** The application was vulnerable to path traversal because functions like `RemoveImage`, `PullImage`, and `InspectImage` replaced `/` and `:` with `-` but failed to reject `.` or `..` inputs. An attacker could pass `..` to effectively target the base directory, causing arbitrary folder deletion or exposure.
**Learning:** Character substitution (e.g. replacing `/` with `-`) is not sufficient for path sandboxing when inputs can still resolve to parent directories like `..` and bypass intended directories.
**Prevention:** Always implement strict input validation to explicitly reject empty strings, `.`, and `..` immediately after any character substitutions and before any filesystem paths are constructed.
