## 2025-02-14 - Fix Zip Slip Vulnerability

**Vulnerability:** Path Traversal (Zip Slip) in tar extraction due to unsafe `filepath.Join` calls.
**Learning:** `filepath.Join(base, filepath.Clean(untrusted))` does not prevent path traversal if `untrusted` contains excessive `../` components. Prefixing the path with `/` forces `filepath.Clean` to evaluate it absolutely, neutralizing escapes.
**Prevention:** Use `filepath.Join(base, filepath.Clean("/" + untrusted))` for safe sandboxed paths.
