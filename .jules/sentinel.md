
## 2024-05-18 - [Path Traversal in Tar Extraction]
**Vulnerability:** Path traversal (Zip Slip / Tar Slip) vulnerability during tar extraction when paths read from headers are joined without preventing escape from target dir using `filepath.Clean`.
**Learning:** `filepath.Join(base, filepath.Clean(untrusted))` is not safe if untrusted contains enough `../` to escape `base`.
**Prevention:** Use `filepath.Join(base, filepath.Clean("/" + untrusted))` to evaluate the untrusted path as absolute before joining.
