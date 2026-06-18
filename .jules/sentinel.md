## 2024-05-18 - [Path Traversal in Tar Extraction]
**Vulnerability:** ZipSlip equivalent in container layer extraction logic where `filepath.Join(base, filepath.Clean(untrusted))` could be bypassed by `../` sequences to overwrite host files outside the intended rootfs directory.
**Learning:** `filepath.Clean` resolves `../` locally but does not bind them to a root directory. When joined with a base directory, it allows path traversal.
**Prevention:** Always convert untrusted inputs to absolute paths relative to a dummy root *before* cleaning them using `filepath.Clean("/" + untrusted)`.
