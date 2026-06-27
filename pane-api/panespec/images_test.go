package panespec

import (
	"testing"
)

func TestRemoveImage_InvalidNames(t *testing.T) {
	// Temporarily override the images dir so we don't accidentally affect real data
	t.Setenv("PANE_IMAGES_DIR", t.TempDir())

	invalidNames := []string{"", ".", "..", "/", "://"}

	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			err := RemoveImage(name)
			if err == nil {
				t.Errorf("Expected error for invalid name %q, got nil", name)
			}
		})
	}
}

func TestInspectImage_InvalidNames(t *testing.T) {
	t.Setenv("PANE_IMAGES_DIR", t.TempDir())

	invalidNames := []string{"", ".", "..", "/", "://"}

	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			_, err := InspectImage(name)
			if err == nil {
				t.Errorf("Expected error for invalid name %q, got nil", name)
			}
		})
	}
}
