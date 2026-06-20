package panespec

import (
	"path/filepath"
	"testing"
)

func TestSecureJoin(t *testing.T) {
	base := "/tmp/rootfs"

	tests := []struct {
		untrusted string
		expected  string
	}{
		{"etc/passwd", "/tmp/rootfs/etc/passwd"},
		{"/etc/passwd", "/tmp/rootfs/etc/passwd"},
		{"../../../etc/passwd", "/tmp/rootfs/etc/passwd"},
		{"a/b/../../../etc/passwd", "/tmp/rootfs/etc/passwd"},
		{"/a/b/../../../etc/passwd", "/tmp/rootfs/etc/passwd"},
		{".wh..wh..opq", "/tmp/rootfs/.wh..wh..opq"},
	}

	for _, tt := range tests {
		actual := secureJoin(base, tt.untrusted)
		// Clean the expected path to ensure cross-platform consistency in tests
		expected := filepath.Clean(tt.expected)
		if actual != expected {
			t.Errorf("secureJoin(%q, %q) = %q, expected %q", base, tt.untrusted, actual, expected)
		}
	}
}
