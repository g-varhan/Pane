
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.
## 2024-05-20 - [Path Traversal in Images Dir Logic]
**Vulnerability:** Path traversal vulnerabilities existed in `pane-api/panespec/images.go`'s image handler functions (`RemoveImage`, `InspectImage`, `PullImage`). Although `/` and `:` were sanitized, inputs like `..` were not. This allowed attackers to perform directory operations like deletion (`os.RemoveAll`) on the parent directory of the images folder.
**Learning:** Go's `filepath.Join(dir, "..")` will resolve to the parent directory of `dir`. Simple character replacement of `/` is insufficient to prevent traversal when the path component itself is entirely `..`.
**Prevention:** Ensure the user input, when evaluated as an absolute path with `filepath.Clean("/" + name)`, is not the root path `/`, which implies the input resolves to empty, `.`, or `..`.
