// SPDX-License-Identifier: Apache-2.0

package panespec

import (
	"testing"
)

func TestValidateImageName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid name", "ubuntu", false},
		{"empty name", "", true},
		{"dot", ".", true},
		{"dot dot", "..", true},
		{"valid complex name", "my-ubuntu-v1.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateImageName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateImageName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRemoveImageTraversal(t *testing.T) {
	// inputs that might lead to path traversal without prefix-stripping
	inputs := []string{".", ".."}
	for _, input := range inputs {
		t.Run("input="+input, func(t *testing.T) {
			err := RemoveImage(input)
			if err == nil {
				t.Errorf("RemoveImage(%q) should return an error, got nil", input)
			}
		})
	}
}

func TestInspectImageTraversal(t *testing.T) {
	inputs := []string{".", ".."}
	for _, input := range inputs {
		t.Run("input="+input, func(t *testing.T) {
			_, err := InspectImage(input)
			if err == nil {
				t.Errorf("InspectImage(%q) should return an error, got nil", input)
			}
		})
	}
}

func TestPullImageTraversal(t *testing.T) {
	// These will strip the "docker://" prefix before evaluating traversal
	inputs := []string{
		"docker://",
		"docker://.",
		"docker://..",
		"pane://",
		"oci://",
	}
	for _, input := range inputs {
		t.Run("input="+input, func(t *testing.T) {
			err := PullImage(input, nil)
			if err == nil {
				t.Errorf("PullImage(%q) should return an error, got nil", input)
			}
		})
	}
}
