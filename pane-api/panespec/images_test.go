package panespec

import (
	"testing"
)

func TestRemoveImageTraversal(t *testing.T) {
	invalidNames := []string{"", ".", ".."}
	for _, name := range invalidNames {
		err := RemoveImage(name)
		if err == nil {
			t.Errorf("expected error for RemoveImage(%q), got nil", name)
		}
	}
}

func TestInspectImageTraversal(t *testing.T) {
	invalidNames := []string{"", ".", ".."}
	for _, name := range invalidNames {
		_, err := InspectImage(name)
		if err == nil {
			t.Errorf("expected error for InspectImage(%q), got nil", name)
		}
	}
}

func TestPullImageTraversal(t *testing.T) {
	invalidNames := []string{"", ".", ".."}
	for _, name := range invalidNames {
		err := PullImage(name, nil)
		if err == nil {
			t.Errorf("expected error for PullImage(%q), got nil", name)
		}
	}
}
