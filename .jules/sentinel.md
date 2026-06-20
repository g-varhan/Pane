
## 2024-05-18 - [Path Traversal in OCI Image Tar Extraction]
**Vulnerability:** Found `filepath.Join(tempRootfsDir, filepath.Clean(header.Name))` used during `.tar` extraction of OCI image layers. The untrusted `header.Name` can contain relative `../` components, allowing a malicious tarball to escape `tempRootfsDir` and overwrite sensitive files (e.g., `/etc/passwd`).
**Learning:** `filepath.Clean` normalizes paths but does not restrict traversal out of the base path unless evaluated as absolute first. In Go, extracting untrusted archives is dangerous without explicit checks.
**Prevention:** Always evaluate untrusted extracted paths as absolute using `filepath.Join(base, filepath.Clean("/" + untrusted))` to safely confine extractions within the target directory boundary.
