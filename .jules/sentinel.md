## 2025-06-19 - Path Traversal Vulnerability in Image Extraction
**Vulnerability:** Path traversal in `PullContainerImage` during tar extraction due to insecure use of `filepath.Join(tempRootfsDir, filepath.Clean(header.Name))` which allows `../` in filenames to escape the temporary directory.
**Learning:** `filepath.Clean` does not remove `../` prefixes if they are evaluated relative to the root, only if they are evaluated inside an absolute path string context first.
**Prevention:** Use a dedicated `secureJoin` function or prepend a `/` before `filepath.Clean` when joining untrusted paths to a trusted base directory to ensure they are rooted.
