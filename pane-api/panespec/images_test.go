package panespec

import (
	"strings"
	"testing"
)

func TestRemoveImage_PathTraversal(t *testing.T) {
	// These names are sanitized. E.g., / -> -
	// So we pass the literal dots.
	tests := []string{".", ".."}

	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			err := RemoveImage(tc)
			if err == nil {
				t.Errorf("RemoveImage(%q) expected error, got nil", tc)
			} else if !strings.Contains(err.Error(), "invalid image name") {
				t.Errorf("RemoveImage(%q) expected 'invalid image name' error, got: %v", tc, err)
			}
		})
	}
}

func TestInspectImage_PathTraversal(t *testing.T) {
	tests := []string{".", ".."}

	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			_, err := InspectImage(tc)
			if err == nil {
				t.Errorf("InspectImage(%q) expected error, got nil", tc)
			} else if !strings.Contains(err.Error(), "invalid image name") {
				t.Errorf("InspectImage(%q) expected 'invalid image name' error, got: %v", tc, err)
			}
		})
	}
}

func TestPullImage_EmptyName(t *testing.T) {
	// PullImage strips prefixes. Just the prefix alone means the name becomes empty.
	tests := []string{
		"docker://",
		"oci://",
		"pane://",
		"",
	}

	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			err := PullImage(tc, nil)
			if err == nil {
				t.Errorf("PullImage(%q) expected error, got nil", tc)
			} else if !strings.Contains(err.Error(), "image name cannot be empty") {
				t.Errorf("PullImage(%q) expected 'image name cannot be empty' error, got: %v", tc, err)
			}
		})
	}
}
