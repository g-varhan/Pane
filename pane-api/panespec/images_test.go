package panespec

import (
	"testing"
)

func TestValidateImageName(t *testing.T) {
	tests := []struct {
		name    string
		imgName string
		wantErr bool
	}{
		{"valid", "ubuntu", false},
		{"empty", "", true},
		{"dot", ".", true},
		{"dotdot", "..", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateImageName(tt.imgName)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateImageName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPullImage_InvalidInput(t *testing.T) {
	// "docker://" strips prefix, becomes empty string
	err := PullImage("docker://", nil)
	if err == nil {
		t.Errorf("PullImage expected error for prefix-only input, got nil")
	}
}

func TestRemoveImage_InvalidInput(t *testing.T) {
	// Test traversal
	err := RemoveImage(".")
	if err == nil {
		t.Errorf("RemoveImage expected error for '.', got nil")
	}
	err = RemoveImage("..")
	if err == nil {
		t.Errorf("RemoveImage expected error for '..', got nil")
	}
}

func TestInspectImage_InvalidInput(t *testing.T) {
	// Test traversal
	_, err := InspectImage(".")
	if err == nil {
		t.Errorf("InspectImage expected error for '.', got nil")
	}
	_, err = InspectImage("..")
	if err == nil {
		t.Errorf("InspectImage expected error for '..', got nil")
	}
}
