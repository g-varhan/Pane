package panespec

import (
	"path/filepath"
)

// secureJoin safely joins a base directory with an untrusted path, preventing path traversal attacks.
func secureJoin(base, untrusted string) string {
	return filepath.Join(base, filepath.Clean("/"+untrusted))
}
