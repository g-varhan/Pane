## 2024-06-18 - Path Traversal in Tar Extraction
**Vulnerability:** ZipSlip / Path Traversal vulnerability during container image extraction using `filepath.Join(tempRootfsDir, filepath.Clean(header.Name))` allowed extracting files outside the intended target directory.
**Learning:** `filepath.Clean` alone does not prevent traversal if the untrusted path uses `../` to navigate up the directory tree past the root folder level when joined.
**Prevention:** Always ensure an untrusted path is evaluated as an absolute path before joining it to a base directory (e.g., `filepath.Join(base, filepath.Clean("/" + untrusted))`).
