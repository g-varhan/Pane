// SPDX-License-Identifier: Apache-2.0

package panespec

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// VMMType represents the VM backend type
type VMMType string

const (
	VMMQemu        VMMType = "qemu"
	VMMFirecracker VMMType = "firecracker"
)

// DiskFormat represents the virtual disk image format
type DiskFormat string

const (
	FormatRaw   DiskFormat = "raw"
	FormatQcow2 DiskFormat = "qcow2"
)

// NetworkMode represents the networking configuration mode
type NetworkMode string

const (
	NetworkNone   NetworkMode = "none"
	NetworkBridge NetworkMode = "bridge"
	NetworkNat    NetworkMode = "nat"
)

type DiskConfig struct {
	Path   *string     `json:"path,omitempty" yaml:"path,omitempty"`
	Size   *string     `json:"size,omitempty" yaml:"size,omitempty"`     // e.g. "10GiB", "20GB"
	Format *DiskFormat `json:"format,omitempty" yaml:"format,omitempty"` // "raw" or "qcow2"
}

type NetworkConfig struct {
	Mode   *NetworkMode `json:"mode,omitempty" yaml:"mode,omitempty"` // "none", "bridge", "nat"
	Bridge *string      `json:"bridge,omitempty" yaml:"bridge,omitempty"`
}

type DriversConfig struct {
	VirtioNet *bool `json:"virtio_net,omitempty" yaml:"virtio_net,omitempty"`
	VirtioBlk *bool `json:"virtio_blk,omitempty" yaml:"virtio_blk,omitempty"`
	VirtioRng *bool `json:"virtio_rng,omitempty" yaml:"virtio_rng,omitempty"`
}

type PaneSpec struct {
	VMM       *VMMType          `json:"vmm,omitempty" yaml:"vmm,omitempty"`
	CPUs      *uint32           `json:"cpus,omitempty" yaml:"cpus,omitempty"`
	Memory    *string           `json:"memory,omitempty" yaml:"memory,omitempty"` // e.g. "512MiB", "2GiB"
	Disk      *DiskConfig       `json:"disk,omitempty" yaml:"disk,omitempty"`
	Image     *string           `json:"image,omitempty" yaml:"image,omitempty"`
	Network   *NetworkConfig    `json:"network,omitempty" yaml:"network,omitempty"`
	Drivers   *DriversConfig    `json:"drivers,omitempty" yaml:"drivers,omitempty"`
	Kernel    *string           `json:"kernel,omitempty" yaml:"kernel,omitempty"`
	Cmdline   *string           `json:"cmdline,omitempty" yaml:"cmdline,omitempty"`
	ExtraArgs []string          `json:"extra_args,omitempty" yaml:"extra_args,omitempty"`
	Env       map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
}

var sizeRegex = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*([a-zA-Z]*)$`)

// ParseSize converts strings like "512MiB" or "10GB" to raw bytes (uint64)
func ParseSize(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty size string")
	}
	matches := sizeRegex.FindStringSubmatch(s)
	if len(matches) != 3 {
		return 0, fmt.Errorf("invalid size format: %q", s)
	}
	val, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse size number: %w", err)
	}
	unit := strings.ToLower(matches[2])
	switch unit {
	case "", "b", "byte", "bytes":
		return uint64(val), nil
	case "k", "kb":
		return uint64(val * 1000), nil
	case "kib":
		return uint64(val * 1024), nil
	case "m", "mb":
		return uint64(val * 1000 * 1000), nil
	case "mib":
		return uint64(val * 1024 * 1024), nil
	case "g", "gb":
		return uint64(val * 1000 * 1000 * 1000), nil
	case "gib":
		return uint64(val * 1024 * 1024 * 1024), nil
	case "t", "tb":
		return uint64(val * 1000 * 1000 * 1000 * 1000), nil
	case "tib":
		return uint64(val * 1024 * 1024 * 1024 * 1024), nil
	default:
		return 0, fmt.Errorf("unknown unit: %q", matches[2])
	}
}

// Validate validates that the fields of the spec have valid values and types
func Validate(spec *PaneSpec) error {
	if spec == nil {
		return errors.New("specification cannot be nil")
	}

	if spec.VMM != nil {
		switch *spec.VMM {
		case VMMQemu, VMMFirecracker:
		default:
			return fmt.Errorf("invalid vmm type: %q (must be %q or %q)", *spec.VMM, VMMQemu, VMMFirecracker)
		}
	}

	if spec.CPUs != nil && *spec.CPUs == 0 {
		return errors.New("cpus must be greater than 0")
	}

	if spec.Memory != nil {
		if _, err := ParseSize(*spec.Memory); err != nil {
			return fmt.Errorf("invalid memory setting: %w", err)
		}
	}

	if spec.Disk != nil {
		if spec.Disk.Format != nil {
			switch *spec.Disk.Format {
			case FormatRaw, FormatQcow2:
			default:
				return fmt.Errorf("invalid disk format: %q (must be %q or %q)", *spec.Disk.Format, FormatRaw, FormatQcow2)
			}
		}
		if spec.Disk.Size != nil {
			if _, err := ParseSize(*spec.Disk.Size); err != nil {
				return fmt.Errorf("invalid disk size setting: %w", err)
			}
		}
		if spec.Disk.Path != nil && *spec.Disk.Path != "" {
			p := *spec.Disk.Path
			if !strings.Contains(p, "://") {
				if _, err := os.Stat(p); err != nil {
					return fmt.Errorf("disk path %q does not exist: %w", p, err)
				}
			}
		}
	}

	if spec.Network != nil {
		if spec.Network.Mode != nil {
			switch *spec.Network.Mode {
			case NetworkNone, NetworkBridge, NetworkNat:
			default:
				return fmt.Errorf("invalid network mode: %q", *spec.Network.Mode)
			}
		}
	}

	if spec.Kernel != nil && *spec.Kernel != "" {
		p := *spec.Kernel
		if !strings.Contains(p, "://") {
			if _, err := os.Stat(p); err != nil {
				return fmt.Errorf("kernel path %q does not exist: %w", p, err)
			}
		}
	}

	return nil
}

// Merge merges base with override. Fields set in override (non-nil) take precedence.
func Merge(base, override *PaneSpec) *PaneSpec {
	if base == nil {
		return override
	}
	if override == nil {
		return base
	}

	result := &PaneSpec{}

	// VMM
	if override.VMM != nil {
		result.VMM = override.VMM
	} else {
		result.VMM = base.VMM
	}

	// CPUs
	if override.CPUs != nil {
		result.CPUs = override.CPUs
	} else {
		result.CPUs = base.CPUs
	}

	// Memory
	if override.Memory != nil {
		result.Memory = override.Memory
	} else {
		result.Memory = base.Memory
	}

	// Disk
	if base.Disk == nil && override.Disk != nil {
		result.Disk = override.Disk
	} else if base.Disk != nil && override.Disk == nil {
		result.Disk = base.Disk
	} else if base.Disk != nil && override.Disk != nil {
		result.Disk = &DiskConfig{}
		if override.Disk.Path != nil {
			result.Disk.Path = override.Disk.Path
		} else {
			result.Disk.Path = base.Disk.Path
		}
		if override.Disk.Size != nil {
			result.Disk.Size = override.Disk.Size
		} else {
			result.Disk.Size = base.Disk.Size
		}
		if override.Disk.Format != nil {
			result.Disk.Format = override.Disk.Format
		} else {
			result.Disk.Format = base.Disk.Format
		}
	}

	// Image
	if override.Image != nil {
		result.Image = override.Image
	} else {
		result.Image = base.Image
	}

	// Network
	if base.Network == nil && override.Network != nil {
		result.Network = override.Network
	} else if base.Network != nil && override.Network == nil {
		result.Network = base.Network
	} else if base.Network != nil && override.Network != nil {
		result.Network = &NetworkConfig{}
		if override.Network.Mode != nil {
			result.Network.Mode = override.Network.Mode
		} else {
			result.Network.Mode = base.Network.Mode
		}
		if override.Network.Bridge != nil {
			result.Network.Bridge = override.Network.Bridge
		} else {
			result.Network.Bridge = base.Network.Bridge
		}
	}

	// Drivers
	if base.Drivers == nil && override.Drivers != nil {
		result.Drivers = override.Drivers
	} else if base.Drivers != nil && override.Drivers == nil {
		result.Drivers = base.Drivers
	} else if base.Drivers != nil && override.Drivers != nil {
		result.Drivers = &DriversConfig{}
		if override.Drivers.VirtioNet != nil {
			result.Drivers.VirtioNet = override.Drivers.VirtioNet
		} else {
			result.Drivers.VirtioNet = base.Drivers.VirtioNet
		}
		if override.Drivers.VirtioBlk != nil {
			result.Drivers.VirtioBlk = override.Drivers.VirtioBlk
		} else {
			result.Drivers.VirtioBlk = base.Drivers.VirtioBlk
		}
		if override.Drivers.VirtioRng != nil {
			result.Drivers.VirtioRng = override.Drivers.VirtioRng
		} else {
			result.Drivers.VirtioRng = base.Drivers.VirtioRng
		}
	}

	// Kernel
	if override.Kernel != nil {
		result.Kernel = override.Kernel
	} else {
		result.Kernel = base.Kernel
	}

	// Cmdline
	if override.Cmdline != nil {
		result.Cmdline = override.Cmdline
	} else {
		result.Cmdline = base.Cmdline
	}

	// ExtraArgs
	result.ExtraArgs = MergeExtraArgs(base.ExtraArgs, override.ExtraArgs)

	// Env
	if base.Env == nil && override.Env != nil {
		result.Env = override.Env
	} else if base.Env != nil && override.Env == nil {
		result.Env = base.Env
	} else if base.Env != nil && override.Env != nil {
		result.Env = make(map[string]string)
		for k, v := range base.Env {
			result.Env[k] = v
		}
		for k, v := range override.Env {
			result.Env[k] = v
		}
	}

	return result
}

func MergeExtraArgs(base, override []string) []string {
	if len(override) == 0 {
		res := make([]string, len(base))
		copy(res, base)
		return res
	}
	if len(base) == 0 {
		res := make([]string, len(override))
		copy(res, override)
		return res
	}

	overrideFlags := make(map[string]bool)
	for i := 0; i < len(override); i++ {
		if strings.HasPrefix(override[i], "-") {
			overrideFlags[override[i]] = true
		}
	}

	result := []string{}
	// Add base args, skipping any flag that is overridden
	for i := 0; i < len(base); {
		arg := base[i]
		if strings.HasPrefix(arg, "-") && overrideFlags[arg] {
			// Skip this flag and its value if it has one and the next token is not a flag
			i++
			if i < len(base) && !strings.HasPrefix(base[i], "-") {
				i++
			}
		} else {
			result = append(result, arg)
			i++
		}
	}

	// Append all override args
	result = append(result, override...)
	return result
}

// Helper functions to get pointers to values
func PtrString(v string) *string                { return &v }
func PtrUint32(v uint32) *uint32                { return &v }
func PtrBool(v bool) *bool                      { return &v }
func PtrVMMType(v VMMType) *VMMType             { return &v }
func PtrDiskFormat(v DiskFormat) *DiskFormat    { return &v }
func PtrNetworkMode(v NetworkMode) *NetworkMode { return &v }

// DefaultProfile returns a panespec matching the legacy hardcoded qemu.c behavior
func DefaultProfile() *PaneSpec {
	return &PaneSpec{
		VMM:    PtrVMMType(VMMQemu),
		CPUs:   PtrUint32(1),
		Memory: PtrString("128MiB"),
		Disk: &DiskConfig{
			Format: PtrDiskFormat(FormatRaw),
		},
		Drivers: &DriversConfig{
			VirtioBlk: PtrBool(true),
			VirtioNet: PtrBool(false),
			VirtioRng: PtrBool(false),
		},
		Network: &NetworkConfig{
			Mode: PtrNetworkMode(NetworkNone),
		},
		ExtraArgs: []string{"-display", "none", "-nographic", "-serial", "none"},
	}
}
