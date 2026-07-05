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
		{"valid name", "ubuntu", false},
		{"valid name with hyphen", "my-ubuntu-image", false},
		{"valid alphanumeric", "alpine3", false},
		{"empty string", "", true},
		{"dot", ".", true},
		{"dot dot", "..", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateImageName(tt.imgName)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateImageName(%q) error = %v, wantErr %v", tt.imgName, err, tt.wantErr)
			}
		})
	}
}

func TestRemoveImage_PathTraversal(t *testing.T) {
	tests := []string{"", ".", ".."}
	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			err := RemoveImage(tt)
			if err == nil {
				t.Errorf("RemoveImage(%q) expected error, got nil", tt)
			}
		})
	}
}

func TestInspectImage_PathTraversal(t *testing.T) {
	tests := []string{"", ".", ".."}
	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			_, err := InspectImage(tt)
			if err == nil {
				t.Errorf("InspectImage(%q) expected error, got nil", tt)
			}
		})
	}
}
