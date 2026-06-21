package panespec

import (
	"testing"
)

func TestSecureJoin(t *testing.T) {
	base := "/tmp/rootfs"
	tests := []struct {
		untrusted string
		expected  string
	}{
		{"../../etc/passwd", "/tmp/rootfs/etc/passwd"},
		{"etc/passwd", "/tmp/rootfs/etc/passwd"},
		{"var/../etc/passwd", "/tmp/rootfs/etc/passwd"},
		{"/etc/passwd", "/tmp/rootfs/etc/passwd"},
	}

	for _, tt := range tests {
		result := secureJoin(base, tt.untrusted)
		if result != tt.expected {
			t.Errorf("secureJoin(%q, %q) = %q, want %q", base, tt.untrusted, result, tt.expected)
		}
	}
}
