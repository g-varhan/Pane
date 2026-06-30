package panespec

import (
	"os"
	"testing"
)

func TestRemoveImage_PathTraversal(t *testing.T) {
	// Create a temporary directory for images
	tmpDir, err := os.MkdirTemp("", "pane-images-test")
	if err != nil {
		t.Fatal(err)
	}
    // Don't defer RemoveAll so we can verify if it was deleted by our function

    os.Setenv("HOME", tmpDir) // mock getImagesDir to use tmpDir

    // Create an image directory to have the images dir
    os.MkdirAll(tmpDir + "/.pane/images", 0755)

    err = RemoveImage("..")
    if err == nil {
        t.Errorf("Expected error for '..'")
    }

    // Check if .pane/images was deleted
    if _, err := os.Stat(tmpDir + "/.pane/images"); os.IsNotExist(err) {
        t.Errorf(".pane/images was deleted, path traversal succeeded")
    }

    // Cleanup
    os.RemoveAll(tmpDir)
}
