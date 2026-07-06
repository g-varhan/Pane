package panespec

import (
	"strings"
	"testing"
)

func TestImageValidation(t *testing.T) {
	tests := []struct {
		name    string
		ref     string
		wantErr bool
	}{
		{
			name:    "empty reference",
			ref:     "",
			wantErr: true,
		},
		{
			name:    "dot",
			ref:     ".",
			wantErr: true,
		},
		{
			name:    "dot dot",
			ref:     "..",
			wantErr: true,
		},
		{
			name:    "docker prefix only",
			ref:     "docker://",
			wantErr: true,
		},
		{
			name:    "pane prefix only",
			ref:     "pane://",
			wantErr: true,
		},
		{
			name:    "valid name",
			ref:     "ubuntu",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run("PullImage_"+tt.name, func(t *testing.T) {
			err := PullImage(tt.ref, nil)
			if (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("PullImage(%q) expected error, got nil", tt.ref)
				} else if strings.Contains(err.Error(), "invalid image name") {
					t.Errorf("PullImage(%q) unexpectedly failed with validation error: %v", tt.ref, err)
				}
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), "invalid image name") {
				t.Errorf("PullImage(%q) expected 'invalid image name' error, got %v", tt.ref, err)
			}
		})

		// RemoveImage and InspectImage are better tested directly
	}
}

func TestImageValidationDirect(t *testing.T) {
	invalidNames := []string{"", ".", ".."}

	for _, name := range invalidNames {
		t.Run("RemoveImage_"+name, func(t *testing.T) {
			err := RemoveImage(name)
			if err == nil || !strings.Contains(err.Error(), "invalid image name") {
				t.Errorf("RemoveImage(%q) expected validation error, got %v", name, err)
			}
		})

		t.Run("InspectImage_"+name, func(t *testing.T) {
			_, err := InspectImage(name)
			if err == nil || !strings.Contains(err.Error(), "invalid image name") {
				t.Errorf("InspectImage(%q) expected validation error, got %v", name, err)
			}
		})
	}
}
