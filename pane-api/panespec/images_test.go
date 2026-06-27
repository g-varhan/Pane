package panespec

import (
	"os"
	"testing"
)

func TestRemoveImage_InvalidNames(t *testing.T) {
	// Setup a temporary directory for images to avoid deleting real system paths if the test fails
	originalTempDir := os.TempDir()
	defer os.Setenv("TMPDIR", originalTempDir)

	tempDir, err := os.MkdirTemp("", "pane_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	os.Setenv("TMPDIR", tempDir)

	invalidNames := []string{"", ".", ".."}

	for _, name := range invalidNames {
		err := RemoveImage(name)
		if err == nil {
			t.Errorf("Expected error for invalid name %q, got nil", name)
		}
	}
}
