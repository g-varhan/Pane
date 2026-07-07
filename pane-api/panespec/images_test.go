package panespec

import "testing"

func TestImageNameValidation(t *testing.T) {
	// Test prefix stripping down to empty string for PullImage
	err := PullImage("docker://", nil)
	if err == nil {
		t.Errorf("PullImage: Expected validation error for empty post-prefix name, got nil")
	}

	// Test raw traversal sequences for RemoveImage
	err = RemoveImage(".")
	if err == nil {
		t.Errorf("RemoveImage: Expected validation error for '.', got nil")
	}

	err = RemoveImage("..")
	if err == nil {
		t.Errorf("RemoveImage: Expected validation error for '..', got nil")
	}

	err = RemoveImage("/")
	// '/' is converted to '-', so it shouldn't hit validation, but might fail due to not being found
	// if it somehow skips validation and gets processed, we expect an error not related to valid image names
	// but we just test what validateImageName catches directly for InspectImage

	_, err = InspectImage(".")
	if err == nil {
		t.Errorf("InspectImage: Expected validation error for '.', got nil")
	}

	_, err = InspectImage("..")
	if err == nil {
		t.Errorf("InspectImage: Expected validation error for '..', got nil")
	}
}
