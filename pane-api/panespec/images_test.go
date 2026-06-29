package panespec

import (
	"testing"
)

func TestRemoveImage_InvalidNames(t *testing.T) {
	// The inputs here bypass standard secure paths, e.g. / => -, : => -, and they get mapped
	// so for instance / gets mapped to -
	// However, if the user directly inputs "", "." or "..", or combinations of "/" that map to ""
	// they should be rejected as invalid image names to protect the host.
	invalidNames := []string{
		"",
		".",
		"..",
		"../", // Becomes ..-
		"/..", // Becomes -..
	}

	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			err := RemoveImage(name)
			if err == nil {
				t.Fatalf("expected error for invalid image name %q, got nil", name)
			}
			if err.Error() != "invalid image name" {
				// We expect a specific validation error, except if it gets mapped to something else
				// and is just not found. Wait, our validation runs AFTER strings.ReplaceAll.
				// So if name == "" or "." or ".." after replace, we expect "invalid image name".
				// Otherwise, we expect "not found" (or similar) but NOT a nil error.

				// In our case, "", ".", ".." will remain themselves.
				if name == "" || name == "." || name == ".." {
					if err.Error() != "invalid image name" {
						t.Errorf("expected 'invalid image name' for %q, got: %v", name, err)
					}
				}
			}
		})
	}
}

func TestInspectImage_InvalidNames(t *testing.T) {
	invalidNames := []string{
		"",
		".",
		"..",
	}

	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			_, err := InspectImage(name)
			if err == nil {
				t.Fatalf("expected error for invalid image name %q, got nil", name)
			}
			if err.Error() != "invalid image name" {
				t.Errorf("expected 'invalid image name' for %q, got: %v", name, err)
			}
		})
	}
}
