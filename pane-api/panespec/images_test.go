package panespec

import (
	"testing"
)

func TestRemoveImage_InvalidNames(t *testing.T) {
	invalidNames := []string{"", ".", "..", "/", "://"}

	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			err := RemoveImage(name)
			if err == nil {
				t.Errorf("expected error for invalid name %q, got nil", name)
			}
		})
	}
}

func TestInspectImage_InvalidNames(t *testing.T) {
	invalidNames := []string{"", ".", "..", "/", "://"}

	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			_, err := InspectImage(name)
			if err == nil {
				t.Errorf("expected error for invalid name %q, got nil", name)
			}
		})
	}
}
