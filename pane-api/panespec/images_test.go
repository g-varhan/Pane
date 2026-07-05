package panespec

import (
	"strings"
	"testing"
)

func TestRemoveImage_Invalid(t *testing.T) {
	tests := []string{
		".",
		"..",
		"",
	}

	for _, tc := range tests {
		err := RemoveImage(tc)
		if err == nil {
			t.Errorf("RemoveImage(%q) expected error, got nil", tc)
		} else if !strings.Contains(err.Error(), "image name cannot be empty") && !strings.Contains(err.Error(), "potential path traversal") {
			t.Errorf("RemoveImage(%q) expected validation error, got: %v", tc, err)
		}
	}
}

func TestInspectImage_Invalid(t *testing.T) {
	tests := []string{
		".",
		"..",
		"",
	}

	for _, tc := range tests {
		_, err := InspectImage(tc)
		if err == nil {
			t.Errorf("InspectImage(%q) expected error, got nil", tc)
		} else if !strings.Contains(err.Error(), "image name cannot be empty") && !strings.Contains(err.Error(), "potential path traversal") {
			t.Errorf("InspectImage(%q) expected validation error, got: %v", tc, err)
		}
	}
}

func TestPullImage_Invalid(t *testing.T) {
	tests := []string{
		".",
		"..",
		"",
		"pane://",
		"docker://",
		"oci://",
	}

	for _, tc := range tests {
		err := PullImage(tc, nil)
		if err == nil {
			t.Errorf("PullImage(%q) expected error, got nil", tc)
		} else if !strings.Contains(err.Error(), "image name cannot be empty") && !strings.Contains(err.Error(), "potential path traversal") {
			t.Errorf("PullImage(%q) expected validation error, got: %v", tc, err)
		}
	}
}
