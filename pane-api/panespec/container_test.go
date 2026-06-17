// SPDX-License-Identifier: Apache-2.0

package panespec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSecureJoin(t *testing.T) {
	tests := []struct {
		name      string
		base      string
		untrusted string
		expected  string
	}{
		{"Normal path", "/tmp/rootfs", "bin/bash", "/tmp/rootfs/bin/bash"},
		{"Absolute path", "/tmp/rootfs", "/etc/passwd", "/tmp/rootfs/etc/passwd"},
		{"Path traversal", "/tmp/rootfs", "../../etc/passwd", "/tmp/rootfs/etc/passwd"},
		{"Complex traversal", "/tmp/rootfs", "var/log/../../../etc/shadow", "/tmp/rootfs/etc/shadow"},
		{"Dot slash", "/tmp/rootfs", "./etc/hosts", "/tmp/rootfs/etc/hosts"},
		{"Trailing slash", "/tmp/rootfs", "var/www/", "/tmp/rootfs/var/www"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := secureJoin(tt.base, tt.untrusted)
			if result != tt.expected {
				t.Errorf("secureJoin(%q, %q) = %q; want %q", tt.base, tt.untrusted, result, tt.expected)
			}
		})
	}
}

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
