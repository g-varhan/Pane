package panespec

import (
	"testing"
)

func TestValidateImageName(t *testing.T) {
	cases := []struct {
		name    string
		wantErr bool
	}{
		{"", true},
		{".", true},
		{"..", true},
		{"valid-image", false},
	}

	for _, tc := range cases {
		err := validateImageName(tc.name)
		if (err != nil) != tc.wantErr {
			t.Errorf("validateImageName(%q) error = %v, wantErr %v", tc.name, err, tc.wantErr)
		}
	}
}

func TestPullImage_Invalid(t *testing.T) {
	// "docker://" reduces to an empty string because PullImage strips the prefix
	err := PullImage("docker://", nil)
	if err == nil {
		t.Errorf("PullImage expected error for prefix-only ref 'docker://', got nil")
	}
}

func TestRemoveImage_Invalid(t *testing.T) {
	err := RemoveImage(".")
	if err == nil {
		t.Errorf("RemoveImage expected error for '.', got nil")
	}

	err = RemoveImage("..")
	if err == nil {
		t.Errorf("RemoveImage expected error for '..', got nil")
	}
}

func TestInspectImage_Invalid(t *testing.T) {
	_, err := InspectImage(".")
	if err == nil {
		t.Errorf("InspectImage expected error for '.', got nil")
	}

	_, err = InspectImage("..")
	if err == nil {
		t.Errorf("InspectImage expected error for '..', got nil")
	}
}
