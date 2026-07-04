package panespec

import (
	"testing"
)

func TestValidateImageName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"ubuntu", false},
		{"", true},
		{".", true},
		{"..", true},
		{"-", false}, // Since "/" gets converted to "-"
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateImageName(tt.name); (err != nil) != tt.wantErr {
				t.Errorf("validateImageName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPullImageValidation(t *testing.T) {
	// Should fail because it trims the prefix and becomes empty
	err := PullImage("docker://", nil)
	if err == nil || err.Error() != "image name cannot be empty" {
		t.Errorf("expected empty image name error, got %v", err)
	}

	err = PullImage("pane://", nil)
	if err == nil || err.Error() != "image name cannot be empty" {
		t.Errorf("expected empty image name error, got %v", err)
	}
}

func TestRemoveImageValidation(t *testing.T) {
	// Path traversal test
	err := RemoveImage("..")
	if err == nil || err.Error() != `invalid image name ".."` {
		t.Errorf("expected invalid image name error for '..', got %v", err)
	}

	err = RemoveImage(".")
	if err == nil || err.Error() != `invalid image name "."` {
		t.Errorf("expected invalid image name error for '.', got %v", err)
	}
}

func TestInspectImageValidation(t *testing.T) {
	// Path traversal test
	_, err := InspectImage("..")
	if err == nil || err.Error() != `invalid image name ".."` {
		t.Errorf("expected invalid image name error for '..', got %v", err)
	}

	_, err = InspectImage(".")
	if err == nil || err.Error() != `invalid image name "."` {
		t.Errorf("expected invalid image name error for '.', got %v", err)
	}
}
