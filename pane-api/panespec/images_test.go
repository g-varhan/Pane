package panespec

import (
	"testing"
)

func TestRemoveImage_InvalidNames(t *testing.T) {
	// These names should be rejected by RemoveImage because they would cause os.RemoveAll
	// to delete the base images directory (or in the case of "/", the name becomes "-" which is valid for the test but handled by not found).

	invalidNames := []string{"", ".", ".."}

	for _, name := range invalidNames {
		err := RemoveImage(name)
		if err == nil {
			t.Errorf("expected error when removing invalid image name %q, got nil", name)
		} else if err.Error() != "invalid image name: \""+name+"\"" {
			t.Errorf("expected 'invalid image name' error for %q, got: %v", name, err)
		}
	}
}

func TestRemoveImage_ValidButNonExistentName(t *testing.T) {
	// For valid names that don't exist, we should get a not found error,
	// rather than the invalid image name error.

	validNonExistentNames := []string{"/", "///", "some-image"}

	for _, name := range validNonExistentNames {
		err := RemoveImage(name)
		if err == nil {
			t.Errorf("expected error when removing non-existent image name %q, got nil", name)
		} else if err.Error() == "invalid image name: \""+name+"\"" {
			t.Errorf("did not expect 'invalid image name' error for %q, got: %v", name, err)
		}
	}
}
