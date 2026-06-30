package panespec

import (
	"testing"
)

func TestRemoveImageInvalidName(t *testing.T) {
	invalidNames := []string{"", ".", "..", "/", ":"} // "/" and ":" become "-" which is valid, but we will test they don't cause traversal

	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			err := RemoveImage(name)

			// For "", ".", ".." we expect our custom error about invalid image name
			if name == "" || name == "." || name == ".." {
				if err == nil {
					t.Fatalf("expected error for invalid name %q, got nil", name)
				}
				if err.Error() != "invalid image name: \""+name+"\"" {
					t.Fatalf("expected specific error for %q, got: %v", name, err)
				}
			} else {
				// For "-" (which "/" and ":" turn into), it will try to remove the image.
				// The result is likely "image \"-\" not found" or similar, but the critical point
				// is that it should NOT delete the whole directory or return nil
				// Let's just make sure it returns an error
				if err == nil {
					t.Fatalf("expected error for invalid name %q, got nil", name)
				}
			}
		})
	}
}

func TestInspectImageInvalidName(t *testing.T) {
	invalidNames := []string{"", ".", "..", "/", ":"}

	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			_, err := InspectImage(name)

			if name == "" || name == "." || name == ".." {
				if err == nil {
					t.Fatalf("expected error for invalid name %q, got nil", name)
				}
				if err.Error() != "invalid image name: \""+name+"\"" {
					t.Fatalf("expected specific error for %q, got: %v", name, err)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error for invalid name %q, got nil", name)
				}
			}
		})
	}
}
