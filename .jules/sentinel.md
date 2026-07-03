
## 2024-05-20 - [Zip Slip Path Traversal]
**Vulnerability:** Tar extraction logic in `pane-api/panespec/container.go` passed untrusted header names to `filepath.Clean` and `filepath.Join`, enabling zip slip vulnerabilities.
**Learning:** `filepath.Join` in Go does not inherently sandbox paths to a specific root directory. If the untrusted input contains `../` sequences that break out of the directory structure when evaluated relative to the current directory, it escapes the expected base path (e.g. `filepath.Join("/base", "../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always prefix untrusted archive entry paths with `/` before calling `filepath.Clean` to force absolute path resolution, trim the leading slash, and then join with the base directory to safely confine extraction.

## 2024-05-20 - [Arbitrary File Deletion via Path Traversal]
**Vulnerability:** In `pane-api/panespec/images.go`, functions handling user-supplied image names used simple character replacements (e.g., changing `/` and `:` to `-`) which mitigated standard path traversals but failed to block `.`, `..`, and empty strings. When passed to `filepath.Join` and `os.RemoveAll`, `..` would cause the deletion of the entire parent directory (`/var/lib/pane`) and `""` would delete the images directory itself.
**Learning:** `strings.ReplaceAll` for path sanitization is insufficient to prevent all path traversals. Explicit validation against literal traversal sequences (`.`, `..`) and empty values is mandatory before using user input in file paths, especially for destructive operations.
**Prevention:** Implement strict input validation to explicitly reject empty strings, `.`, and `..` before path construction, rather than relying solely on sanitization or string replacements.
