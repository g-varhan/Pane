
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## 2026-07-06 - [Path Traversal via Image Names]
**Vulnerability:** Functions managing images (`PullImage`, `RemoveImage`, `InspectImage`) did not validate if image names constructed into file paths were strictly empty, `.`, or `..`. While `strings.ReplaceAll` mitigated `/` and `:`, it allowed `..` to traverse to parent directories during deletion.
**Learning:** Simple string replacement is insufficient against raw traversal strings like `.` and `..`. Relying on sandboxing functions like `secureJoin` fails when the final constructed path for deletion resolves exactly to the base directory itself (e.g. deleting the root images folder).
**Prevention:** Strictly validate any user-provided string used as a single path segment against empty strings, `.`, and `..` before it is joined with base directories.
