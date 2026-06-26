package panespec

import (
	"strings"
	"testing"
)

func TestRemoveImage_PathTraversal(t *testing.T) {
	invalidNames := []string{"", ".", ".."}

	for _, name := range invalidNames {
		err := RemoveImage(name)
		if err == nil {
			t.Errorf("expected error for invalid image name %q, got nil", name)
		} else if !strings.Contains(err.Error(), "invalid image name") {
			t.Errorf("expected 'invalid image name' error for %q, got: %v", name, err)
		}
	}
}

func TestInspectImage_PathTraversal(t *testing.T) {
	invalidNames := []string{"", ".", ".."}

	for _, name := range invalidNames {
		_, err := InspectImage(name)
		if err == nil {
			t.Errorf("expected error for invalid image name %q, got nil", name)
		} else if !strings.Contains(err.Error(), "invalid image name") {
			t.Errorf("expected 'invalid image name' error for %q, got: %v", name, err)
		}
	}
}

func TestPullImage_PathTraversal(t *testing.T) {
	invalidNames := []string{"", ".", ".."}

	for _, name := range invalidNames {
		err := PullImage(name, nil)
		if err == nil {
			t.Errorf("expected error for invalid image name %q, got nil", name)
		} else if !strings.Contains(err.Error(), "invalid image name") {
			t.Errorf("expected 'invalid image name' error for %q, got: %v", name, err)
		}
	}
}
