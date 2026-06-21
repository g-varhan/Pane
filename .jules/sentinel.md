## 2024-05-24 - Path Traversal in Go's filepath.Join
**Vulnerability:** Path traversal when joining a base path with user-supplied untrusted path using `filepath.Join(base, filepath.Clean(untrusted))`.
**Learning:** `filepath.Clean` evaluates `../` components, but doesn't prevent them from escaping the base path if the untrusted path is relative.
**Prevention:** Ensure the untrusted path is evaluated as an absolute path before joining with the base path. Example: `filepath.Join(base, filepath.Clean("/" + untrusted))`.
