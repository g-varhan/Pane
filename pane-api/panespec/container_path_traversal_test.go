package panespec

import (
	"path/filepath"
	"testing"
)

func TestSecureJoin(t *testing.T) {
	baseDir := "/var/lib/pane/images/tmp"

	tests := []struct {
		untrusted string
		expected  string
	}{
		{
			untrusted: "foo/bar.txt",
			expected:  filepath.Join(baseDir, "foo/bar.txt"),
		},
		{
			untrusted: "/foo/bar.txt",
			expected:  filepath.Join(baseDir, "foo/bar.txt"),
		},
		{
			untrusted: "../../../../etc/passwd",
			expected:  filepath.Join(baseDir, "etc/passwd"),
		},
		{
			untrusted: "foo/../../../../etc/passwd",
			expected:  filepath.Join(baseDir, "etc/passwd"),
		},
		{
			untrusted: "./././foo.txt",
			expected:  filepath.Join(baseDir, "foo.txt"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.untrusted, func(t *testing.T) {
			result := secureJoin(baseDir, tc.untrusted)
			if result != tc.expected {
				t.Errorf("secureJoin(%q, %q) = %q; want %q", baseDir, tc.untrusted, result, tc.expected)
			}
		})
	}
}
