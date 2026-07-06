package panespec

import (
	"testing"
)

func TestPullImage_InvalidInput(t *testing.T) {
	invalidInputs := []string{"pane://", "docker://.", "oci://.."}
	for _, input := range invalidInputs {
		err := PullImage(input, nil)
		if err == nil {
			t.Errorf("expected error for invalid input %q, got nil", input)
		}
	}
}

func TestRemoveImage_InvalidInput(t *testing.T) {
	invalidInputs := []string{"", ".", ".."}
	for _, input := range invalidInputs {
		err := RemoveImage(input)
		if err == nil {
			t.Errorf("expected error for invalid input %q, got nil", input)
		}
	}
}

func TestInspectImage_InvalidInput(t *testing.T) {
	invalidInputs := []string{"", ".", ".."}
	for _, input := range invalidInputs {
		_, err := InspectImage(input)
		if err == nil {
			t.Errorf("expected error for invalid input %q, got nil", input)
		}
	}
}
