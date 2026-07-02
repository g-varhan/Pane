package panespec

import (
	"os"
	"testing"
)

func TestValidateImageName(t *testing.T) {
	tests := []struct {
		name    string
		imgName string
		wantErr bool
	}{
		{"Empty string", "", true},
		{"Dot", ".", true},
		{"Double dot", "..", true},
		{"Valid name", "ubuntu", false},
		{"Converted slash", "-", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateImageName(tt.imgName)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateImageName(%q) error = %v, wantErr %v", tt.imgName, err, tt.wantErr)
			}
		})
	}
}

func TestRemoveImagePathTraversal(t *testing.T) {
	// Create a temporary images directory for testing
	tmpDir, err := os.MkdirTemp("", "pane-test-images-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Override getImagesDir temporarily by manipulating environment variables or
	// we just test the function directly since it's hard to mock getImagesDir.
	// We just want to check if RemoveImage returns an error for invalid input.

	invalidInputs := []string{"", ".", "..", "/"}

	for _, input := range invalidInputs {
		t.Run("Input "+input, func(t *testing.T) {
			err := RemoveImage(input)
			if err == nil {
				// The only exception is "/", because it gets converted to "-"
				// and if the image "-" doesn't exist, it should return "image not found"
				// but NOT nil.
				t.Errorf("RemoveImage(%q) expected error, got nil", input)
			}

			// For "/", since it becomes "-", it should NOT give an invalid image error,
			// but rather a "not found" error because "-" doesn't exist.
			if input == "/" && err != nil && err.Error() == "invalid image name: \"-\"" {
				t.Errorf("RemoveImage(%q) should not return invalid name error, it was converted to %q", input, "-")
			}

			// For empty string, it should return an error related to empty image name.
			if input == "" && err != nil && err.Error() != "image name cannot be empty" {
				t.Errorf("RemoveImage(%q) expected empty image name error, got: %v", input, err)
			}
		})
	}
}

func TestInspectImagePathTraversal(t *testing.T) {
	invalidInputs := []string{"", ".", ".."}
	for _, input := range invalidInputs {
		t.Run("Input "+input, func(t *testing.T) {
			_, err := InspectImage(input)
			if err == nil {
				t.Errorf("InspectImage(%q) expected error, got nil", input)
			}
		})
	}
}

func TestPullImagePathTraversal(t *testing.T) {
	invalidInputs := []string{"", ".", ".."}
	for _, input := range invalidInputs {
		t.Run("Input "+input, func(t *testing.T) {
			err := PullImage(input, nil)
			if err == nil {
				t.Errorf("PullImage(%q) expected error, got nil", input)
			}
		})
	}
}
