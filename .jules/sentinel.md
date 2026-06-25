
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## 2026-06-25 - [Image Handling Path Traversal]
**Vulnerability:** `RemoveImage`, `InspectImage`, and `PullImage` in `pane-api/panespec/images.go` constructed directory paths using `filepath.Join` without sufficiently sandboxing the `name` argument, which could contain `..` or leading slashes not stripped by `strings.ReplaceAll` alone. This enables path traversal where operations could manipulate files outside the `/var/lib/pane/images` base directory.
**Learning:** Simple string substitutions like `strings.ReplaceAll(name, "/", "-")` do not protect against `..` components being evaluated by `filepath.Join()`, leading to directory escape.
**Prevention:** Always sandbox path construction when incorporating user-supplied file names by routing them through a secure join function (like evaluating `filepath.Clean("/" + untrusted)`) before concatenating them with a base directory.
