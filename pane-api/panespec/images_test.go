// SPDX-License-Identifier: Apache-2.0

package panespec

import (
	"strings"
	"testing"
)

func TestImageNameValidation(t *testing.T) {
	// These cases should reduce to invalid names after sanitization and trigger the validation error.
	invalidCases := []struct {
		desc string
		ref  string
	}{
		{"empty string", ""},
		{"current directory", "."},
		{"parent directory", ".."},
		{"docker prefix only", "docker://"},
		{"oci prefix only", "oci://"},
		{"pane prefix only", "pane://"},
	}

	for _, tc := range invalidCases {
		t.Run("PullImage_"+tc.desc, func(t *testing.T) {
			err := PullImage(tc.ref, nil)
			if err == nil {
				t.Errorf("PullImage(%q) expected error, got nil", tc.ref)
			} else if !strings.Contains(err.Error(), "invalid image name") {
				t.Errorf("PullImage(%q) expected 'invalid image name' error, got: %v", tc.ref, err)
			}
		})
	}

	// RemoveImage and InspectImage don't strip URI prefixes, so we test raw traversal strings.
	rawTraversalCases := []struct {
		desc string
		name string
	}{
		{"empty string", ""},
		{"current directory", "."},
		{"parent directory", ".."},
	}

	for _, tc := range rawTraversalCases {
		t.Run("RemoveImage_"+tc.desc, func(t *testing.T) {
			err := RemoveImage(tc.name)
			if err == nil {
				t.Errorf("RemoveImage(%q) expected error, got nil", tc.name)
			} else if !strings.Contains(err.Error(), "invalid image name") {
				t.Errorf("RemoveImage(%q) expected 'invalid image name' error, got: %v", tc.name, err)
			}
		})

		t.Run("InspectImage_"+tc.desc, func(t *testing.T) {
			_, err := InspectImage(tc.name)
			if err == nil {
				t.Errorf("InspectImage(%q) expected error, got nil", tc.name)
			} else if !strings.Contains(err.Error(), "invalid image name") {
				t.Errorf("InspectImage(%q) expected 'invalid image name' error, got: %v", tc.name, err)
			}
		})
	}
}
