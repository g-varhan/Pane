package panespec

import (
	"strings"
	"testing"
)

// Test for security regression: path traversal and empty names
func TestRemoveImage_InvalidNames(t *testing.T) {
	// These names should trigger validateImageName errors directly
	tests := []string{"", ".", ".."}
	// Prefix-only names reduce to empty strings during PullImage,
	// but for Remove/Inspect they don't have the prefix stripping.
	// So for RemoveImage we just test the raw traversal names.
	for _, name := range tests {
		err := RemoveImage(name)
		if err == nil || (!strings.Contains(err.Error(), "image name cannot be empty") && !strings.Contains(err.Error(), "invalid image name")) {
			t.Errorf("RemoveImage(%q) expected validation error, got %v", name, err)
		}
	}
}

func TestInspectImage_InvalidNames(t *testing.T) {
	tests := []string{"", ".", ".."}
	for _, name := range tests {
		_, err := InspectImage(name)
		if err == nil || (!strings.Contains(err.Error(), "image name cannot be empty") && !strings.Contains(err.Error(), "invalid image name")) {
			t.Errorf("InspectImage(%q) expected validation error, got %v", name, err)
		}
	}
}

func TestPullImage_InvalidNames(t *testing.T) {
	// PullImage strips prefixes. These will reduce to empty strings and fail validation.
	tests := []string{"", ".", "..", "docker://", "oci://", "pane://"}
	for _, name := range tests {
		// Provide a dummy func to avoid nil panic if validation somehow passes
		dummyFunc := func(string, string) error { return nil }
		err := PullImage(name, dummyFunc)
		if err == nil || (!strings.Contains(err.Error(), "image name cannot be empty") && !strings.Contains(err.Error(), "invalid image name")) {
			t.Errorf("PullImage(%q) expected validation error, got %v", name, err)
		}
	}
}
