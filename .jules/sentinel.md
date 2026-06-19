## 2025-02-18 - Path Traversal Vulnerability in `filepath.Clean`

**Vulnerability:**
Using `filepath.Join(base, filepath.Clean(untrusted))` for extracting files is vulnerable to path traversal if the untrusted path contains `../` sequences that exceed the base directory level. The `filepath.Clean` function does not collapse `../` sequences that resolve outside of the passed string's "root" if it's evaluated as a relative path. This allowed extraction of OCI image layers to overwrite files outside of the intended root filesystem directory.

**Learning:**
`filepath.Clean` only removes `../` components when they don't exceed the top level of the path provided to it. If an attacker provides a relative path like `../../../etc/passwd`, `filepath.Clean` will return `../../../etc/passwd`. `filepath.Join` will then append this directly to the base, escaping the sandbox. By prepending `/` before cleaning (i.e. `filepath.Clean("/" + untrusted)`), the path is evaluated as an absolute path, and any `../` attempting to go above the root (`/`) are safely collapsed.

**Prevention:**
Always use a dedicated `secureJoin` function when appending untrusted file paths to a base directory:
```go
func secureJoin(base, untrusted string) string {
    cleanPath := filepath.Clean("/" + untrusted)
    relPath := strings.TrimPrefix(cleanPath, "/")
    return filepath.Join(base, relPath)
}
```
