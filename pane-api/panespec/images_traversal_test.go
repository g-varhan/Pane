package panespec

import (
	"testing"
)

func TestPathTraversalPrevention(t *testing.T) {
	err := RemoveImage("..")
	if err == nil {
		t.Fatal("Expected error for RemoveImage(\"..\"), got nil")
	}

	_, err = InspectImage("..")
	if err == nil {
		t.Fatal("Expected error for InspectImage(\"..\"), got nil")
	}

	err = PullImage("..", nil)
	if err == nil {
		t.Fatal("Expected error for PullImage(\"..\"), got nil")
	}
}
