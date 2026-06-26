package panespec

import (
	"strings"
	"testing"
)

func TestRemoveImage_PathTraversal(t *testing.T) {
	tests := []string{"", ".", "..", "docker://", "pane://"}
	for _, tt := range tests {
		t.Run("name="+tt, func(t *testing.T) {
			err := RemoveImage(tt)
			if err == nil {
				t.Errorf("expected error for image name %q, got nil", tt)
			} else if !strings.Contains(err.Error(), "invalid image name") && !strings.Contains(err.Error(), "not found") {
				// We expect either invalid image name or not found, but not a successful removal of base dir
				t.Logf("got error: %v", err)
			}
		})
	}
}

func TestInspectImage_PathTraversal(t *testing.T) {
	tests := []string{"", ".", "..", "docker://", "pane://"}
	for _, tt := range tests {
		t.Run("name="+tt, func(t *testing.T) {
			_, err := InspectImage(tt)
			if err == nil {
				t.Errorf("expected error for image name %q, got nil", tt)
			} else if !strings.Contains(err.Error(), "invalid image name") && !strings.Contains(err.Error(), "not found") {
				// We expect either invalid image name or not found
				t.Logf("got error: %v", err)
			}
		})
	}
}

func TestPullImage_PathTraversal(t *testing.T) {
	tests := []string{"", ".", "..", "docker://", "pane://"}
	for _, tt := range tests {
		t.Run("name="+tt, func(t *testing.T) {
			err := PullImage(tt, nil)
			if err == nil {
				t.Errorf("expected error for image name %q, got nil", tt)
			} else if !strings.Contains(err.Error(), "invalid image name") && !strings.Contains(err.Error(), "unknown image reference") {
				// We expect either invalid image name or unknown image reference
				t.Logf("got error: %v", err)
			}
		})
	}
}
