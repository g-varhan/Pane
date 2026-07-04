package panespec

import (
	"strings"
	"testing"
)

func TestImageNameValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty string", "", true},
		{"current dir", ".", true},
		{"parent dir", "..", true},
		{"valid name", "ubuntu", false},
		{"valid name with slash", "library/ubuntu", false},
		{"valid name with colon", "ubuntu:latest", false},
	}

	for _, tt := range tests {
		t.Run("PullImage_"+tt.name, func(t *testing.T) {
			// PullImage strips prefixes before validation
			input := tt.input
			// If input is just a prefix, stripping it results in empty string
			if input == "docker://" || input == "oci://" || input == "pane://" {
				// The test runner expects this to be caught by empty string validation
			}

			err := PullImage(input, nil)
			if (err != nil && strings.Contains(err.Error(), "image name cannot be empty")) ||
				(err != nil && strings.Contains(err.Error(), "invalid image name")) {
				if !tt.wantErr {
					t.Errorf("PullImage(%q) returned unexpected validation error: %v", input, err)
				}
			} else if tt.wantErr {
				t.Errorf("PullImage(%q) expected validation error, got: %v", input, err)
			}
		})

		t.Run("RemoveImage_"+tt.name, func(t *testing.T) {
			err := RemoveImage(tt.input)
			if (err != nil && strings.Contains(err.Error(), "image name cannot be empty")) ||
				(err != nil && strings.Contains(err.Error(), "invalid image name")) {
				if !tt.wantErr {
					t.Errorf("RemoveImage(%q) returned unexpected validation error: %v", tt.input, err)
				}
			} else if tt.wantErr {
				t.Errorf("RemoveImage(%q) expected validation error, got: %v", tt.input, err)
			}
		})

		t.Run("InspectImage_"+tt.name, func(t *testing.T) {
			_, err := InspectImage(tt.input)
			if (err != nil && strings.Contains(err.Error(), "image name cannot be empty")) ||
				(err != nil && strings.Contains(err.Error(), "invalid image name")) {
				if !tt.wantErr {
					t.Errorf("InspectImage(%q) returned unexpected validation error: %v", tt.input, err)
				}
			} else if tt.wantErr {
				t.Errorf("InspectImage(%q) expected validation error, got: %v", tt.input, err)
			}
		})
	}

	// Test prefix stripping logic in PullImage
	t.Run("PullImage_PrefixStrippingToEmpty", func(t *testing.T) {
		prefixes := []string{"docker://", "oci://", "pane://"}
		for _, prefix := range prefixes {
			err := PullImage(prefix, nil)
			if err == nil || !strings.Contains(err.Error(), "image name cannot be empty") {
				t.Errorf("PullImage(%q) should have failed with empty string error, got: %v", prefix, err)
			}
		}
	})
}
