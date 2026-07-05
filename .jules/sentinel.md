
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## 2026-07-05 - [Path Traversal in Destructive Operations]
**Vulnerability:** The `RemoveImage` and `InspectImage` functions in `pane-api/panespec/images.go` constructed target directory paths using `filepath.Join` with an untrusted image `name` string. A user providing `name=".."` would bypass the slash/colon sanitization and cause `os.RemoveAll` to delete the entire parent directory containing all images, leading to arbitrary file deletion and data loss.
**Learning:** Simple string replacement (`strings.ReplaceAll(name, "/", "-")`) is insufficient for sanitizing identifiers that will be used in filesystem paths. It specifically fails to protect against raw `.` and `..` traversal sequences that don't include slashes. `filepath.Join(baseDir, "..")` resolves validly to the parent directory, making destructive operations extremely dangerous.
**Prevention:** Always validate user-supplied identifiers (like image names, container IDs, or usernames) used in file paths against a strict allowlist of characters, or explicitly reject dangerous inputs like empty strings (`""`), `.` and `..` before path construction, especially before calling `os.RemoveAll`.
