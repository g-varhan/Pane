## 2023-10-27 - Zip Slip Vulnerability in Container Image Extraction

**Vulnerability:** A Zip Slip (Path Traversal) vulnerability was identified in `pane-api/panespec/container.go` during the extraction of container layers. The code used `filepath.Join` blindly with `header.Name` from tar archives, allowing malicious paths (e.g., `../../../../etc/passwd`) to escape the intended extraction directory and overwrite host files.

**Learning:** `filepath.Join` does not guarantee the resulting path is securely confined within the base directory, especially when the target path contains `../` or starts with `/`.

**Prevention:** Implement a `secureJoin` function that verifies the final joined path is strictly within the expected base directory (e.g., using `strings.HasPrefix(joined, cleanBase + string(filepath.Separator))`). Always use this for paths extracted from archives like tar or zip.
