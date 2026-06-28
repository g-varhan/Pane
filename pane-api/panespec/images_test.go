package panespec

import (
	"testing"
)

func TestRemoveImage_InvalidNames(t *testing.T) {
	invalidNames := []string{"", ".", ".."}

	for _, name := range invalidNames {
		err := RemoveImage(name)
		if err == nil {
			t.Errorf("Expected error when removing image with name %q, got nil", name)
		}
	}
}

func TestInspectImage_InvalidNames(t *testing.T) {
	invalidNames := []string{"", ".", ".."}

	for _, name := range invalidNames {
		_, err := InspectImage(name)
		if err == nil {
			t.Errorf("Expected error when inspecting image with name %q, got nil", name)
		}
	}
}

func TestPullImage_InvalidNames(t *testing.T) {
	invalidNames := []string{"", ".", "..", "pane://", "docker://", "oci://"}

	for _, name := range invalidNames {
		err := PullImage(name, nil)
		if err == nil {
			t.Errorf("Expected error when pulling image with name %q, got nil", name)
		}
	}
}
