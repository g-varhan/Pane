package panespec

import (
	"testing"
)

func TestRemoveImage(t *testing.T) {
	// Let's test the path traversal vulnerability with invalid inputs
	err := RemoveImage("..")
	if err == nil || err.Error() != "invalid image name: must not be empty or contain path traversal characters" {
		t.Errorf("Expected an error when trying to remove '..', got: %v", err)
	}

	err = RemoveImage(".")
	if err == nil || err.Error() != "invalid image name: must not be empty or contain path traversal characters" {
		t.Errorf("Expected an error when trying to remove '.', got: %v", err)
	}

	err = RemoveImage("")
	if err == nil || err.Error() != "invalid image name: must not be empty or contain path traversal characters" {
		t.Errorf("Expected an error when trying to remove '', got: %v", err)
	}
}

func TestInspectImage(t *testing.T) {
	_, err := InspectImage("..")
	if err == nil || err.Error() != "invalid image name: must not be empty or contain path traversal characters" {
		t.Errorf("Expected an error when trying to inspect '..', got: %v", err)
	}

	_, err = InspectImage(".")
	if err == nil || err.Error() != "invalid image name: must not be empty or contain path traversal characters" {
		t.Errorf("Expected an error when trying to inspect '.', got: %v", err)
	}

	_, err = InspectImage("")
	if err == nil || err.Error() != "invalid image name: must not be empty or contain path traversal characters" {
		t.Errorf("Expected an error when trying to inspect '', got: %v", err)
	}
}
