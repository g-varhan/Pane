// SPDX-License-Identifier: Apache-2.0

package panespec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSize(t *testing.T) {
	tests := []struct {
		input    string
		expected uint64
		hasError bool
	}{
		{"512MiB", 512 * 1024 * 1024, false},
		{"2GiB", 2 * 1024 * 1024 * 1024, false},
		{"10GB", 10 * 1000 * 1000 * 1000, false},
		{"100B", 100, false},
		{"1.5MiB", 1.5 * 1024 * 1024, false},
		{"", 0, true},
		{"abc", 0, true},
		{"-5MiB", 0, true},
	}

	for _, tc := range tests {
		res, err := ParseSize(tc.input)
		if tc.hasError {
			if err == nil {
				t.Errorf("expected error for input %q, got nil", tc.input)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error for input %q: %v", tc.input, err)
			}
			if res != tc.expected {
				t.Errorf("expected %d for %q, got %d", tc.expected, tc.input, res)
			}
		}
	}
}

func TestValidate(t *testing.T) {
	// Setup temporary files to test path checks
	tmpDir, err := os.MkdirTemp("", "panespec-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	existingPath := filepath.Join(tmpDir, "exist.img")
	if err := os.WriteFile(existingPath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	nonExistingPath := filepath.Join(tmpDir, "nonexist.img")

	tests := []struct {
		name     string
		spec     *PaneSpec
		hasError bool
	}{
		{
			"valid default profile",
			DefaultProfile(),
			false,
		},
		{
			"invalid VMM",
			&PaneSpec{VMM: PtrVMMType("virtualbox")},
			true,
		},
		{
			"invalid CPUs",
			&PaneSpec{CPUs: PtrUint32(0)},
			true,
		},
		{
			"invalid memory format",
			&PaneSpec{Memory: PtrString("invalid")},
			true,
		},
		{
			"invalid disk format",
			&PaneSpec{Disk: &DiskConfig{Format: PtrDiskFormat("vmdk")}},
			true,
		},
		{
			"valid disk path (existing file)",
			&PaneSpec{Disk: &DiskConfig{Path: PtrString(existingPath)}},
			false,
		},
		{
			"invalid disk path (non-existing local file)",
			&PaneSpec{Disk: &DiskConfig{Path: PtrString(nonExistingPath)}},
			true,
		},
		{
			"valid disk path (remote scheme)",
			&PaneSpec{Disk: &DiskConfig{Path: PtrString("pane://ubuntu")}},
			false,
		},
		{
			"invalid network mode",
			&PaneSpec{Network: &NetworkConfig{Mode: PtrNetworkMode("invalid")}},
			true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.spec)
			if tc.hasError && err == nil {
				t.Error("expected validation error, got nil")
			}
			if !tc.hasError && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestMerge(t *testing.T) {
	defaults := DefaultProfile()

	imageMetadata := &PaneSpec{
		CPUs:   PtrUint32(2),
		Memory: PtrString("512MiB"),
		Disk: &DiskConfig{
			Path:   PtrString("pane://ubuntu"),
			Format: PtrDiskFormat(FormatQcow2),
		},
		Drivers: &DriversConfig{
			VirtioNet: PtrBool(true),
		},
	}

	fileConfig := &PaneSpec{
		CPUs:   PtrUint32(4),
		Memory: PtrString("1GiB"),
	}

	cliFlags := &PaneSpec{
		CPUs: PtrUint32(8),
	}

	// 1. Defaults vs Image Metadata
	merged1 := Merge(defaults, imageMetadata)
	if *merged1.CPUs != 2 {
		t.Errorf("expected CPUs to be 2 from image metadata, got %d", *merged1.CPUs)
	}
	if *merged1.Memory != "512MiB" {
		t.Errorf("expected memory to be 512MiB, got %s", *merged1.Memory)
	}
	if *merged1.Disk.Format != FormatQcow2 {
		t.Errorf("expected format to be qcow2, got %s", *merged1.Disk.Format)
	}
	if *merged1.Drivers.VirtioNet != true {
		t.Errorf("expected virtio_net to be true, got %t", *merged1.Drivers.VirtioNet)
	}
	if *merged1.Drivers.VirtioBlk != true {
		t.Errorf("expected virtio_blk to be true (inherited), got %t", *merged1.Drivers.VirtioBlk)
	}

	// 2. Add File Config
	merged2 := Merge(merged1, fileConfig)
	if *merged2.CPUs != 4 {
		t.Errorf("expected CPUs to be 4 from file config, got %d", *merged2.CPUs)
	}
	if *merged2.Memory != "1GiB" {
		t.Errorf("expected memory to be 1GiB from file config, got %s", *merged2.Memory)
	}
	if *merged2.Disk.Format != FormatQcow2 {
		t.Errorf("expected disk format to remain qcow2, got %s", *merged2.Disk.Format)
	}

	// 3. Add CLI Flags (highest precedence)
	merged3 := Merge(merged2, cliFlags)
	if *merged3.CPUs != 8 {
		t.Errorf("expected CPUs to be 8 from CLI flags, got %d", *merged3.CPUs)
	}
	if *merged3.Memory != "1GiB" {
		t.Errorf("expected memory to remain 1GiB, got %s", *merged3.Memory)
	}
}

func TestConfigInitAndValidate(t *testing.T) {
	scaffold := ConfigInit()
	if scaffold == "" {
		t.Fatal("empty scaffold config")
	}

	tmpDir, err := os.MkdirTemp("", "panespec-init-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "pane.yaml")
	if err := os.WriteFile(configPath, []byte(scaffold), 0644); err != nil {
		t.Fatal(err)
	}

	spec, err := ConfigValidate(configPath)
	if err != nil {
		t.Fatalf("failed to validate scaffold config: %v", err)
	}

	if *spec.VMM != VMMQemu {
		t.Errorf("expected vmm to be qemu, got %s", *spec.VMM)
	}
	if *spec.CPUs != 1 {
		t.Errorf("expected cpus to be 1, got %d", *spec.CPUs)
	}
	if *spec.Memory != "128MiB" {
		t.Errorf("expected memory to be 128MiB, got %s", *spec.Memory)
	}
}
