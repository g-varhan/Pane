package panespec

import (
	"testing"
)

func TestValidateImageName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"", true},
		{".", true},
		{"..", true},
		{"validname", false},
		{"foo-bar", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateImageName(tt.name); (err != nil) != tt.wantErr {
				t.Errorf("validateImageName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestRemoveImage_InvalidNames(t *testing.T) {
	invalidNames := []string{"", ".", ".."}

	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			err := RemoveImage(name)
			if err == nil || err.Error() != "invalid image name: \".\"" && err.Error() != "invalid image name: \"..\"" && err.Error() != "image name cannot be empty" {
				t.Errorf("RemoveImage(%q) should return a validation error, got: %v", name, err)
			}
		})
	}
}

func TestInspectImage_InvalidNames(t *testing.T) {
	invalidNames := []string{"", ".", ".."}

	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			_, err := InspectImage(name)
			if err == nil || err.Error() != "invalid image name: \".\"" && err.Error() != "invalid image name: \"..\"" && err.Error() != "image name cannot be empty" {
				t.Errorf("InspectImage(%q) should return a validation error, got: %v", name, err)
			}
		})
	}
}

func TestPullImage_InvalidNames(t *testing.T) {
	invalidNames := []string{"pane://", "pane://.", "pane://..", "docker://", "oci://"}

	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			err := PullImage(name, func(string, string) error { return nil })
			if err == nil || err.Error() != "invalid image name: \".\"" && err.Error() != "invalid image name: \"..\"" && err.Error() != "image name cannot be empty" {
				t.Errorf("PullImage(%q) should return a validation error, got: %v", name, err)
			}
		})
	}
}
