package panespec

import (
	"strings"
	"testing"
)

func TestInvalidImageNames(t *testing.T) {
	invalidNames := []string{"", ".", ".."}
	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			err := RemoveImage(name)
			if err == nil || !strings.Contains(err.Error(), "invalid image name") {
				t.Errorf("RemoveImage(%q) expected 'invalid image name' error, got: %v", name, err)
			}

			_, err = InspectImage(name)
			if err == nil || !strings.Contains(err.Error(), "invalid image name") {
				t.Errorf("InspectImage(%q) expected 'invalid image name' error, got: %v", name, err)
			}

			err = PullImage(name, func(_, _ string) error { return nil })
			if err == nil || !strings.Contains(err.Error(), "invalid image name") {
				t.Errorf("PullImage(%q) expected 'invalid image name' error, got: %v", name, err)
			}
		})
	}
}
