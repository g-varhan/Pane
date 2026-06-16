## 2025-02-14 - Fix Path Traversal in OCI Image Tar Extraction
**Vulnerability:** Path traversal vulnerability when extracting tar layers inside `PullContainerImage` due to unsafe usage of `filepath.Join(tempRootfsDir, filepath.Clean(header.Name))` which does not sanitize relative traversals (`../`) effectively escaping the rootfs sandbox.
**Learning:** `filepath.Clean` doesn't remove `../` if the path still stays relative or allows escaping a base dir when concatenated using `filepath.Join` blindly.
**Prevention:** Always securely sandbox untrusted user paths against a base by either wrapping them as an absolute string before cleaning `filepath.Clean("/" + untrusted)` and validating containment, or ensuring it evaluates natively within the sandbox.
