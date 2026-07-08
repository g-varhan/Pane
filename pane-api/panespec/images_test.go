package panespec

import (
	"testing"
)

func TestValidateImageName(t *testing.T) {
	tests := []struct {
		name    string
		img     string
		wantErr bool
	}{
		{"empty", "", true},
		{"dot", ".", true},
		{"dotdot", "..", true},
		{"valid", "ubuntu", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateImageName(tt.img)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateImageName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPullImage_Invalid(t *testing.T) {
	err := PullImage("docker://", nil)
	if err == nil {
		t.Errorf("Expected error for prefix-only string")
	}
}

func TestRemoveImage_Invalid(t *testing.T) {
	err := RemoveImage("..")
	if err == nil {
		t.Errorf("Expected error for '..'")
	}
	err = RemoveImage("")
	if err == nil {
		t.Errorf("Expected error for empty string")
	}
}

func TestInspectImage_Invalid(t *testing.T) {
	_, err := InspectImage("..")
	if err == nil {
		t.Errorf("Expected error for '..'")
	}
}
