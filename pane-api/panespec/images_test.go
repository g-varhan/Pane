package panespec

import (
	"testing"
)

func TestRemoveImage_InvalidNames(t *testing.T) {
	invalidNames := []string{"", ".", ".."}
	for _, name := range invalidNames {
		err := RemoveImage(name)
		if err == nil {
			t.Errorf("expected error for invalid image name %q, got nil", name)
		}
	}
}
