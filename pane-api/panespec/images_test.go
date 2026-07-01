package panespec

import (
	"strings"
	"testing"
)

func TestRemoveImage_InvalidNames(t *testing.T) {
	// These names after sanitization by strings.ReplaceAll("/", "-")
	// should still hit the validateImageName logic.
	// Actually "/" becomes "-", which is a valid image name (a directory named "-").
	// But "", ".", and ".." are unmodified and must be rejected.

	tests := []struct {
		name string
	}{
		{name: ""},
		{name: "."},
		{name: ".."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RemoveImage(tt.name)
			if err == nil {
				t.Errorf("RemoveImage(%q) expected error, got nil", tt.name)
			} else if !strings.Contains(err.Error(), "invalid image name") {
				t.Errorf("RemoveImage(%q) expected 'invalid image name' error, got: %v", tt.name, err)
			}
		})
	}
}

func TestInspectImage_InvalidNames(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: ""},
		{name: "."},
		{name: ".."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := InspectImage(tt.name)
			if err == nil {
				t.Errorf("InspectImage(%q) expected error, got nil", tt.name)
			} else if !strings.Contains(err.Error(), "invalid image name") {
				t.Errorf("InspectImage(%q) expected 'invalid image name' error, got: %v", tt.name, err)
			}
		})
	}
}
