package panespec

import (
	"testing"
)

func TestValidateImageName(t *testing.T) {
	// These names are what gets passed to validateImageName after the ReplaceAll
	// operations in PullImage, RemoveImage, and InspectImage.
	tests := []struct {
		name    string
		isValid bool
	}{
		{"", false},
		{".", false},
		{"..", false},
		{"valid-name", true},
		{"my-image-tag", true},
		{"-", true}, // Represents "/" or ":" after replacement
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateImageName(tc.name)
			if tc.isValid && err != nil {
				t.Fatalf("expected valid name %q, got error: %v", tc.name, err)
			}
			if !tc.isValid && err == nil {
				t.Fatalf("expected error for invalid name %q, but got nil", tc.name)
			}
		})
	}
}

func TestImageOperationsWithInvalidNames(t *testing.T) {
	invalidInputs := []string{
		"",
		".",
		"..",
		"docker://",
		"oci://",
		"pane://",
	}

	for _, input := range invalidInputs {
		t.Run("RemoveImage_"+input, func(t *testing.T) {
			err := RemoveImage(input)
			if err == nil {
				t.Errorf("RemoveImage(%q): expected error, got nil", input)
			}
		})

		t.Run("InspectImage_"+input, func(t *testing.T) {
			_, err := InspectImage(input)
			if err == nil {
				t.Errorf("InspectImage(%q): expected error, got nil", input)
			}
		})

		t.Run("PullImage_"+input, func(t *testing.T) {
			err := PullImage(input, func(a, b string) error { return nil })
			if err == nil {
				t.Errorf("PullImage(%q): expected error, got nil", input)
			}
		})
	}
}
