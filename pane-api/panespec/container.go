// SPDX-License-Identifier: Apache-2.0

package panespec

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"golang.org/x/sys/unix"
)

// PullContainerImage pulls a Docker/OCI image, flattens layers, injects init + agent,
// formats it to an ext4 raw disk image, and downloads the Firecracker guest kernel.
func PullContainerImage(ref, targetDir string) error {
	// 1. Check filesystem (fail if cache dir is on ext4)
	if err := checkFilesystem(targetDir); err != nil {
		return err
	}

	imageRef := strings.TrimPrefix(ref, "docker://")
	imageRef = strings.TrimPrefix(imageRef, "oci://")

	repoRef, err := name.ParseReference(imageRef)
	if err != nil {
		return fmt.Errorf("failed to parse reference %q: %w", imageRef, err)
	}

	fmt.Printf("Fetching image metadata for %s...\n", repoRef.Name())
	img, err := remote.Image(repoRef)
	if err != nil {
		return fmt.Errorf("failed to fetch image: %w", err)
	}

	configFile, err := img.ConfigFile()
	if err != nil {
		return fmt.Errorf("failed to get image config: %w", err)
	}

	// Create temp directory to build the rootfs
	tempRootfsDir, err := os.MkdirTemp("", "pane-rootfs-*")
	if err != nil {
		return fmt.Errorf("failed to create temp rootfs dir: %w", err)
	}
	defer os.RemoveAll(tempRootfsDir)

	// Pull and extract layers
	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("failed to get image layers: %w", err)
	}

	fmt.Printf("Extracting %d layers...\n", len(layers))
	for i, layer := range layers {
		fmt.Printf("Extracting layer %d/%d...\n", i+1, len(layers))
		rc, err := layer.Compressed()
		if err != nil {
			return fmt.Errorf("failed to get layer reader: %w", err)
		}

		// Decompress layer if gzipped
		var reader io.Reader = rc
		gr, err := gzip.NewReader(rc)
		if err == nil {
			reader = gr
		}

		tr := tar.NewReader(reader)
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				rc.Close()
				if gr != nil {
					gr.Close()
				}
				return fmt.Errorf("failed to read tar header: %w", err)
			}

			// Resolve target path
			target, err := secureJoin(tempRootfsDir, header.Name)
			if err != nil {
				rc.Close()
				if gr != nil {
					gr.Close()
				}
				return fmt.Errorf("invalid path in tar archive: %w", err)
			}

			// Handle whiteouts (.wh.*)
			base := filepath.Base(header.Name)
			dir := filepath.Dir(header.Name)
			if strings.HasPrefix(base, ".wh.") {
				if base == ".wh..wh..opq" {
					// Opaque whiteout: delete all contents of the directory
					targetDirToDelete, err := secureJoin(tempRootfsDir, dir)
					if err != nil {
						continue // skip invalid whiteout paths
					}
					entries, err := os.ReadDir(targetDirToDelete)
					if err == nil {
						for _, entry := range entries {
							_ = os.RemoveAll(filepath.Join(targetDirToDelete, entry.Name()))
						}
					}
				} else {
					// Single file whiteout: delete the target file
					fileToDelete := strings.TrimPrefix(base, ".wh.")
					dirToDelete, err := secureJoin(tempRootfsDir, dir)
					if err == nil {
						_ = os.RemoveAll(filepath.Join(dirToDelete, fileToDelete))
					}
				}
				continue
			}

			switch header.Typeflag {
			case tar.TypeDir:
				if err := os.MkdirAll(target, 0755); err != nil {
					rc.Close()
					if gr != nil {
						gr.Close()
					}
					return fmt.Errorf("failed to create dir: %w", err)
				}
			case tar.TypeReg:
				_ = os.MkdirAll(filepath.Dir(target), 0755)
				f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, header.FileInfo().Mode())
				if err != nil {
					rc.Close()
					if gr != nil {
						gr.Close()
					}
					return fmt.Errorf("failed to create file %s: %w", target, err)
				}
				if _, err := io.Copy(f, tr); err != nil {
					f.Close()
					rc.Close()
					if gr != nil {
						gr.Close()
					}
					return fmt.Errorf("failed to write file: %w", err)
				}
				f.Close()
			case tar.TypeSymlink:
				_ = os.MkdirAll(filepath.Dir(target), 0755)
				_ = os.Remove(target)
				if err := os.Symlink(header.Linkname, target); err != nil {
					rc.Close()
					if gr != nil {
						gr.Close()
					}
					return fmt.Errorf("failed to create symlink: %w", err)
				}
			case tar.TypeLink:
				_ = os.MkdirAll(filepath.Dir(target), 0755)
				_ = os.Remove(target)
				oldPath, err := secureJoin(tempRootfsDir, header.Linkname)
				if err != nil {
					rc.Close()
					if gr != nil {
						gr.Close()
					}
					return fmt.Errorf("invalid link target path in tar archive: %w", err)
				}
				if err := os.Link(oldPath, target); err != nil {
					rc.Close()
					if gr != nil {
						gr.Close()
					}
					return fmt.Errorf("failed to create hard link %s -> %s: %w", oldPath, target, err)
				}
			}
		}
		if gr != nil {
			gr.Close()
		}
		rc.Close()
	}

	// 2. Inject vsock agent (pane-agent) and custom init
	agentSrc := os.Getenv("PANE_AGENT_PATH")
	if agentSrc == "" {
		agentSrc = "/usr/local/bin/pane-agent"
		if _, err := os.Stat(agentSrc); os.IsNotExist(err) {
			found := false
			for _, p := range []string{
				"pane-core/pane-agent",
				"../pane-core/pane-agent",
				"../../pane-core/pane-agent",
				"/var/lib/pane/pane-agent",
			} {
				if _, err := os.Stat(p); err == nil {
					agentSrc = p
					found = true
					break
				}
			}
			if !found {
				agentSrc = "pane-core/pane-agent"
			}
		}
	}

	usrSbin := filepath.Join(tempRootfsDir, "usr/sbin")
	_ = os.MkdirAll(usrSbin, 0755)
	if err := copyFile(agentSrc, filepath.Join(usrSbin, "pane-agent")); err != nil {
		return fmt.Errorf("failed to inject pane-agent: %w", err)
	}
	_ = os.Chmod(filepath.Join(usrSbin, "pane-agent"), 0755)

	// Compile and inject static init
	initSrc := os.Getenv("PANE_INIT_SRC_PATH")
	if initSrc == "" {
		initSrc = "pane-api/panespec/init_src/init.c"
		if _, err := os.Stat(initSrc); os.IsNotExist(err) {
			found := false
			for _, p := range []string{
				"panespec/init_src/init.c",
				"init_src/init.c",
				"../pane-api/panespec/init_src/init.c",
				"../../pane-api/panespec/init_src/init.c",
				"/var/lib/pane/init.c",
			} {
				if _, err := os.Stat(p); err == nil {
					initSrc = p
					found = true
					break
				}
			}
			if !found {
				initSrc = "pane-api/panespec/init_src/init.c"
			}
		}
	}
	initDst := filepath.Join(tempRootfsDir, "init")
	fmt.Printf("Compiling static init from %s...\n", initSrc)
	cmdBuild := exec.Command("gcc", "-static", "-O2", initSrc, "-o", initDst)
	if out, err := cmdBuild.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to compile static init: %v (output: %s)", err, string(out))
	}

	// Write OCI configuration to /oci-config.json
	type OciConfig struct {
		Entrypoint []string `json:"entrypoint,omitempty"`
		Cmd        []string `json:"cmd,omitempty"`
		Env        []string `json:"env,omitempty"`
		Workdir    string   `json:"workdir,omitempty"`
		User       string   `json:"user,omitempty"`
	}
	cfg := OciConfig{
		Entrypoint: configFile.Config.Entrypoint,
		Cmd:        configFile.Config.Cmd,
		Env:        configFile.Config.Env,
		Workdir:    configFile.Config.WorkingDir,
		User:       configFile.Config.User,
	}
	cfgData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize OCI config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tempRootfsDir, "oci-config.json"), cfgData, 0644); err != nil {
		return fmt.Errorf("failed to write oci-config.json: %w", err)
	}

	// Make sure /etc/resolv.conf is populated so dns works in the guest
	etcDir := filepath.Join(tempRootfsDir, "etc")
	_ = os.MkdirAll(etcDir, 0755)
	_ = os.WriteFile(filepath.Join(etcDir, "resolv.conf"), []byte("nameserver 8.8.8.8\n"), 0644)

	// 3. Estimate rootfs size and convert to ext4 using mke2fs
	diskPath := filepath.Join(targetDir, "disk.raw")
	_ = os.Remove(diskPath)

	sizeBytes, err := dirSize(tempRootfsDir)
	if err != nil {
		return fmt.Errorf("failed to calculate rootfs size: %w", err)
	}
	sizeMB := int64(float64(sizeBytes) / (1024 * 1024) * 1.5)
	if sizeMB < 128 {
		sizeMB = 128
	}

	fmt.Printf("Creating ext4 raw disk image (%d MB) from rootfs...\n", sizeMB)
	// Create empty raw file first
	f, err := os.Create(diskPath)
	if err != nil {
		return err
	}
	if err := f.Truncate(sizeMB * 1024 * 1024); err != nil {
		f.Close()
		return err
	}
	f.Close()

	// Format with mke2fs -d
	cmdFormat := exec.Command("mke2fs", "-d", tempRootfsDir, "-t", "ext4", diskPath)
	if out, err := cmdFormat.CombinedOutput(); err != nil {
		return fmt.Errorf("mke2fs failed: %v (output: %s)", err, string(out))
	}

	// 4. Download Firecracker guest kernel (uncompressed ELF vmlinux)
	kernelPath := filepath.Join(targetDir, "vmlinuz")
	if _, err := os.Stat(kernelPath); os.IsNotExist(err) {
		kernelUrl := "https://s3.amazonaws.com/spec.ccfc.min/img/quickstart_guide/x86_64/kernels/vmlinux.bin"
		fmt.Printf("Downloading Firecracker guest kernel from %s...\n", kernelUrl)
		if err := downloadFile(kernelUrl, kernelPath); err != nil {
			return fmt.Errorf("failed to download guest kernel: %w", err)
		}
	}

	// 5. Write metadata.json and panespec.json
	meta := ImageMetadata{
		Name:       filepath.Base(filepath.Dir(targetDir)),
		Version:    "latest",
		VMM:        "firecracker",
		Source:     ref,
		KernelPath: kernelPath,
	}
	metaData, _ := json.MarshalIndent(meta, "", "  ")
	_ = os.WriteFile(filepath.Join(targetDir, "metadata.json"), metaData, 0644)

	spec := DefaultProfile()
	spec.VMM = PtrVMMType(VMMFirecracker)
	spec.CPUs = PtrUint32(1)
	spec.Memory = PtrString("256MiB")
	spec.Disk = &DiskConfig{
		Path:   PtrString(diskPath),
		Format: PtrDiskFormat(FormatRaw),
	}
	spec.Kernel = PtrString(kernelPath)
	spec.Cmdline = PtrString("console=ttyS0 reboot=k panic=1 pci=off")
	spec.Drivers = &DriversConfig{
		VirtioNet: PtrBool(true),
		VirtioBlk: PtrBool(true),
		VirtioRng: PtrBool(false),
	}
	specData, _ := json.MarshalIndent(spec, "", "  ")
	_ = os.WriteFile(filepath.Join(targetDir, "panespec.json"), specData, 0644)

	fmt.Printf("Successfully converted %s OCI container to bootable Pane image!\n", ref)
	return nil
}

func secureJoin(base, path string) (string, error) {
	base = filepath.Clean(base)

	// Make the path relative to root by prepending / and cleaning.
	// This evaluates dot-dot components within the path itself,
	// effectively chrooting it to / so it can't break out.
	// E.g., "/../../etc/passwd" -> "/etc/passwd"
	cleanedPath := strings.TrimPrefix(filepath.Clean("/"+path), "/")

	joined := filepath.Join(base, cleanedPath)

	// Paranoia check: verify that joined still starts with base
	if !strings.HasPrefix(joined, base+string(filepath.Separator)) && joined != base {
		return "", fmt.Errorf("path traversal attempt: %s", path)
	}
	return joined, nil
}

func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func checkFilesystem(path string) error {
	var stat unix.Statfs_t
	checkPath := path
	for {
		if _, err := os.Stat(checkPath); err == nil {
			break
		}
		checkPath = filepath.Dir(checkPath)
		if checkPath == "/" || checkPath == "." {
			break
		}
	}
	if err := unix.Statfs(checkPath, &stat); err != nil {
		return err
	}
	// EXT4_SUPER_MAGIC is 0xEF53
	if stat.Type == 0xEF53 {
		return fmt.Errorf("conversion cache requires btrfs or xfs. Configure with PANE_IMAGE_CACHE=/path/on/btrfs")
	}
	return nil
}
