
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## 2024-07-06 - [Path Traversal in Image Deletion and Inspection]
**Vulnerability:** The `RemoveImage` and `InspectImage` functions in `pane-api/panespec/images.go` constructed paths via `filepath.Join(dir, name)` without rejecting traversal strings like `.` or `..`, leading to arbitrary directory deletion (e.g. deleting the base image directory).
**Learning:** Functions evaluating file system paths from user-provided identifiers without an inherent sandbox limit require explicit string-level validation against empty strings, `.`, and `..` immediately after sanitizing. `filepath.Join` safely joins `..` by removing it if it's trapped within the base path, but not when passed explicitly.
**Prevention:** Always explicitly check for and reject empty strings or direct traversal sequences (`.`, `..`) in input path identifiers prior to any `filepath.Join` or `os.RemoveAll` logic.
