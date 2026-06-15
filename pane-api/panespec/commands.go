// SPDX-License-Identifier: Apache-2.0

package panespec

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// ConfigInit returns the scaffolded commented YAML configuration
func ConfigInit() string {
	return `# Pane VM configuration file (pane.yaml)
# For more information, see https://github.com/pane-vmm/pane

# VMM backend selector (qemu | firecracker)
vmm: qemu

# Number of virtual CPUs
cpus: 1

# Memory allocation (e.g. 512MiB, 2GiB)
memory: 128MiB

# Disk configuration
disk:
  # Path to the disk image file
  # path: /path/to/disk.img
  # Disk format (raw | qcow2)
  format: raw
  # Optional disk resize size (e.g., 20GiB)
  # size: 20GiB

# Network configuration
network:
  # Mode (none | bridge | nat)
  mode: none
  # Bridge interface name (optional, used when mode is bridge)
  # bridge: br0

# Guest drivers to enable
drivers:
  virtio_net: false
  virtio_blk: true
  virtio_rng: false

# Direct-kernel-boot options (optional)
# kernel: /path/to/vmlinuz
# cmdline: console=ttyS0 reboot=k panic=1

# Raw QEMU command line escape hatch arguments (QEMU-only)
# extra_args:
#   - "-display"
#   - "none"
`
}

// ConfigValidate reads a configuration file (YAML or JSON) and validates it
func ConfigValidate(filePath string) (*PaneSpec, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	spec := &PaneSpec{}
	// yaml.Unmarshal parses both JSON and YAML automatically
	if err := yaml.Unmarshal(content, spec); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := Validate(spec); err != nil {
		return spec, fmt.Errorf("validation failed: %w", err)
	}

	return spec, nil
}
