
## 2025-02-17 - Go filepath.Join Path Traversal via filepath.Clean
**Vulnerability:** A Zip Slip / Tar Slip path traversal vulnerability occurs when extracting an archive and directly concatenating the base directory with `filepath.Clean(header.Name)` inside `filepath.Join`.
**Learning:** `filepath.Join` evaluates relative paths. If the untrusted path is `../../../../etc/passwd`, `filepath.Clean("..")` keeps the `..`, which then traverses outside the base directory. `filepath.Join(base, filepath.Clean(untrusted))` is NOT safe.
**Prevention:** Always prepend a `/` to the untrusted input before cleaning it, forcing it to be evaluated as an absolute path. E.g. `filepath.Join(base, filepath.Clean("/"+untrusted))`. This sandboxes the path resolution strictly inside the base directory.
