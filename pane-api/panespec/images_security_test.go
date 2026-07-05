package panespec

import (
	"strings"
	"testing"
)

func TestImageValidationSecurity(t *testing.T) {
	// 1. Test RemoveImage path traversal prevention
	tests := []string{"", ".", ".."}

	for _, tc := range tests {
		t.Run("RemoveImage_"+tc, func(t *testing.T) {
			err := RemoveImage(tc)
			if err == nil {
				t.Fatalf("RemoveImage(%q) should have failed", tc)
			}

			// Verify it failed because of our validation, not an underlying os.Stat error
			if !strings.Contains(err.Error(), "cannot be empty") && !strings.Contains(err.Error(), "invalid image name") {
				t.Errorf("RemoveImage(%q) failed with unexpected error: %v", tc, err)
			}
		})

		t.Run("InspectImage_"+tc, func(t *testing.T) {
			_, err := InspectImage(tc)
			if err == nil {
				t.Fatalf("InspectImage(%q) should have failed", tc)
			}
			if !strings.Contains(err.Error(), "cannot be empty") && !strings.Contains(err.Error(), "invalid image name") {
				t.Errorf("InspectImage(%q) failed with unexpected error: %v", tc, err)
			}
		})
	}

	// 2. Test PullImage where the prefix stripping results in an empty string
	emptyPrefixes := []string{"pane://", "docker://", "oci://"}

	for _, prefix := range emptyPrefixes {
		t.Run("PullImage_Empty_"+prefix, func(t *testing.T) {
			err := PullImage(prefix, nil)
			if err == nil {
				t.Fatalf("PullImage(%q) should have failed when prefix is stripped to empty string", prefix)
			}
			if !strings.Contains(err.Error(), "cannot be empty") {
				t.Errorf("PullImage(%q) failed with unexpected error: %v", prefix, err)
			}
		})
	}
}
