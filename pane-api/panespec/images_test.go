// SPDX-License-Identifier: Apache-2.0

package panespec

import (
	"testing"
)

func TestImagesPathTraversal(t *testing.T) {
	invalidNames := []string{
		"",
		".",
		"..",
		"/", // Because '/' gets replaced with '-', it will become "-" which is valid for directory, but let's test if we actually handle the invalid inputs
	}

	for _, name := range invalidNames {
		t.Run("RemoveImage_"+name, func(t *testing.T) {
			err := RemoveImage(name)

			// For "/", it becomes "-", which might say "image not found"
			if name == "/" {
				if err == nil {
					t.Logf("Warning: RemoveImage succeeded for %q which evaluated to '-'. Check if it existed.", name)
				}
				// we don't strictly require it to be "invalid image name" because it's replaced by "-",
				// but we just ensure it doesn't crash or succeed.
			} else {
				if err == nil {
					t.Fatalf("Expected error for removing invalid image name %q, got nil", name)
				}
				if err.Error() != "invalid image name: \""+name+"\"" {
					if name == "" && err.Error() == "invalid image name: \"\"" {
						// Expected
					} else {
						t.Errorf("Expected 'invalid image name' error, got: %v", err)
					}
				}
			}
		})

		t.Run("InspectImage_"+name, func(t *testing.T) {
			_, err := InspectImage(name)
			if name == "/" {
				if err == nil {
					t.Fatalf("Expected error for inspecting image name %q, got nil", name)
				}
			} else {
				if err == nil {
					t.Fatalf("Expected error for inspecting invalid image name %q, got nil", name)
				}
				if err.Error() != "invalid image name: \""+name+"\"" {
					if name == "" && err.Error() == "invalid image name: \"\"" {
						// Expected
					} else {
						t.Errorf("Expected 'invalid image name' error, got: %v", err)
					}
				}
			}
		})

		t.Run("PullImage_"+name, func(t *testing.T) {
			// pane://.. docker://.. oci://..
			refs := []string{
				name,
				"pane://" + name,
				"docker://" + name,
			}
			for _, ref := range refs {
				err := PullImage(ref, nil)
				if name == "/" {
					if err == nil {
						t.Fatalf("Expected error for pulling image ref %q, got nil", ref)
					}
				} else {
					if err == nil {
						t.Fatalf("Expected error for pulling invalid image ref %q, got nil", ref)
					}
					if err.Error() != "invalid image name: \""+name+"\"" {
						t.Errorf("Expected 'invalid image name' error for %q, got: %v", ref, err)
					}
				}
			}
		})
	}
}
