// SPDX-License-Identifier: Apache-2.0

package panespec

import (
	"strings"
	"testing"
)

func TestRemoveImage_PathTraversal(t *testing.T) {
	tests := []struct {
		name    string
		imgName string
		wantErr bool
	}{
		{"Empty string", "", true},
		{"Dot", ".", true},
		{"Dot dot", "..", true},
		{"Dot dot with slash", "../", true}, // This tests if we expect it to fail, but it's not path traversal, it's not found
		{"Dot dot with colon", "..:", true}, // This tests if we expect it to fail, but it's not path traversal, it's not found
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RemoveImage(tt.imgName)

			// The tests for "../" and "..:" might return a "not found" error, not a path traversal error.
			// Since we just want to ensure it doesn't do a path traversal and instead safely fails (either
			// because it's caught as invalid, or because the sanitized name "..-" is just treated as a
			// non-existent image), we accept any error.

			if (err != nil) != tt.wantErr {
				t.Errorf("RemoveImage(%q) error = %v, wantErr %v", tt.imgName, err, tt.wantErr)
				return
			}

			// For purely traversal paths, ensure the specific error message is present
			if tt.imgName == "" || tt.imgName == "." || tt.imgName == ".." {
				if tt.wantErr && err != nil && !strings.Contains(err.Error(), "invalid image name") {
					t.Errorf("RemoveImage(%q) error = %v, want error containing 'invalid image name'", tt.imgName, err)
				}
			}
		})
	}
}

func TestInspectImage_PathTraversal(t *testing.T) {
	tests := []struct {
		name    string
		imgName string
		wantErr bool
	}{
		{"Empty string", "", true},
		{"Dot", ".", true},
		{"Dot dot", "..", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := InspectImage(tt.imgName)
			if (err != nil) != tt.wantErr {
				t.Errorf("InspectImage(%q) error = %v, wantErr %v", tt.imgName, err, tt.wantErr)
				return
			}

			// If wantErr is true, ensure it's specifically for invalid name and not just "not found"
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), "invalid image name") {
				t.Errorf("InspectImage(%q) error = %v, want error containing 'invalid image name'", tt.imgName, err)
			}
		})
	}
}
