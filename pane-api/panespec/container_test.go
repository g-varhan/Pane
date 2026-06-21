// SPDX-License-Identifier: Apache-2.0

package panespec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPullContainerImage(t *testing.T) {
	// Skip test if not running on Linux or if mke2fs is not available
	if _, err := os.Stat("/usr/bin/mke2fs"); os.IsNotExist(err) {
		t.Skip("mke2fs not available, skipping container pull test")
	}

	tempDir, err := os.MkdirTemp("", "pane-container-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Pull and convert busybox (extremely small image)
	ref := "docker://busybox:latest"
	err = PullContainerImage(ref, tempDir)
	if err != nil {
		t.Fatalf("PullContainerImage failed: %v", err)
	}

	// Verify outputs
	diskPath := filepath.Join(tempDir, "disk.raw")
	if _, err := os.Stat(diskPath); os.IsNotExist(err) {
		t.Errorf("disk.raw not created")
	}

	kernelPath := filepath.Join(tempDir, "vmlinuz")
	if _, err := os.Stat(kernelPath); os.IsNotExist(err) {
		t.Errorf("vmlinuz not created")
	}

	metadataPath := filepath.Join(tempDir, "metadata.json")
	metaData, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("failed to read metadata.json: %v", err)
	}
	var meta ImageMetadata
	if err := json.Unmarshal(metaData, &meta); err != nil {
		t.Fatalf("failed to parse metadata.json: %v", err)
	}
	if meta.VMM != "firecracker" {
		t.Errorf("expected VMM firecracker, got %s", meta.VMM)
	}
	if meta.Source != ref {
		t.Errorf("expected source %s, got %s", ref, meta.Source)
	}

	specPath := filepath.Join(tempDir, "panespec.json")
	spec, err := ConfigValidate(specPath)
	if err != nil {
		t.Fatalf("failed to parse and validate panespec.json: %v", err)
	}
	if string(*spec.VMM) != "firecracker" {
		t.Errorf("expected spec VMM firecracker, got %s", *spec.VMM)
	}
	if *spec.Disk.Path != diskPath {
		t.Errorf("expected spec disk path %s, got %s", diskPath, *spec.Disk.Path)
	}
}

func TestSecureJoin(t *testing.T) {
	base := "/tmp/pane-rootfs"

	tests := []struct {
		name      string
		untrusted string
		expected  string
	}{
		{
			name:      "Normal path",
			untrusted: "usr/bin/test",
			expected:  "/tmp/pane-rootfs/usr/bin/test",
		},
		{
			name:      "Path traversal attempt",
			untrusted: "../../../../etc/passwd",
			expected:  "/tmp/pane-rootfs/etc/passwd",
		},
		{
			name:      "Absolute path attempt",
			untrusted: "/etc/passwd",
			expected:  "/tmp/pane-rootfs/etc/passwd",
		},
		{
			name:      "Whiteout file",
			untrusted: ".wh.some_file",
			expected:  "/tmp/pane-rootfs/.wh.some_file",
		},
		{
			name:      "Complex traversal",
			untrusted: "var/lib/../../../../etc/shadow",
			expected:  "/tmp/pane-rootfs/etc/shadow",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := secureJoin(base, tc.untrusted)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}
