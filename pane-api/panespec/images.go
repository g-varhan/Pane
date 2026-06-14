package panespec

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type ImageMetadata struct {
	Name            string   `json:"name"`
	Version         string   `json:"version"`
	VMM             string   `json:"vmm"`
	Source          string   `json:"source"`
	KernelPath      string   `json:"kernel_path,omitempty"`
	DriversRequired []string `json:"drivers_required,omitempty"`
}

func getImagesDir() string {
	dir := "/var/lib/pane/images"
	if err := os.MkdirAll(dir, 0755); err != nil {
		dir = filepath.Join(os.TempDir(), "pane/images")
	}
	return dir
}

type ImageInfo struct {
	Metadata ImageMetadata `json:"metadata"`
	Size     int64         `json:"size"`
}

// PullImage resolves the ref (manifest, local, http, container) and registers it
func PullImage(ref string, ctrPullFunc func(string, string) error) error {
	name := strings.TrimPrefix(ref, "pane://")
	name = strings.TrimPrefix(name, "docker://")
	name = strings.TrimPrefix(name, "oci://")

	// Clean up image name for filesystem compatibility
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, ":", "-")

	imagesDir := getImagesDir()
	targetDir := filepath.Join(imagesDir, name, "v1.0")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create image dir: %w", err)
	}

	diskPath := filepath.Join(targetDir, "disk.iso")

	// 1. Check if it's the tiny10 Windows test image
	if name == "tiny10" {
		srcIso := "/home/varhan/Documents/disk/tiny10 x64 23h2.iso"
		if _, err := os.Stat(srcIso); os.IsNotExist(err) {
			return fmt.Errorf("tiny10 test ISO not found at %s", srcIso)
		}

		fmt.Printf("Registering tiny10 test ISO via symlink...\n")
		_ = os.Remove(diskPath)
		if err := os.Symlink(srcIso, diskPath); err != nil {
			// Fallback to copy if symlink fails
			if err := copyFile(srcIso, diskPath); err != nil {
				return fmt.Errorf("failed to copy tiny10 ISO: %w", err)
			}
		}

		// Write metadata.json
		meta := ImageMetadata{
			Name:    "tiny10",
			Version: "v1.0",
			VMM:     "qemu",
			Source:  "local://" + srcIso,
		}
		metaData, _ := json.MarshalIndent(meta, "", "  ")
		_ = os.WriteFile(filepath.Join(targetDir, "metadata.json"), metaData, 0644)

		// Write panespec.json
		spec := DefaultProfile()
		spec.VMM = PtrVMMType(VMMQemu)
		spec.CPUs = PtrUint32(4)
		spec.Memory = PtrString("4GiB")
		spec.Disk = &DiskConfig{
			Path:   PtrString(diskPath),
			Format: PtrDiskFormat(FormatRaw),
		}
		spec.Drivers = &DriversConfig{
			VirtioNet: PtrBool(true),
			VirtioBlk: PtrBool(true),
			VirtioRng: PtrBool(false),
		}
		spec.ExtraArgs = []string{
			"-cdrom",
			"/home/varhan/Documents/disk/virtio-win-0.1.271.iso",
			"-vnc",
			":1",
		}
		specData, _ := json.MarshalIndent(spec, "", "  ")
		_ = os.WriteFile(filepath.Join(targetDir, "panespec.json"), specData, 0644)

		fmt.Println("tiny10 registered successfully!")
		return nil
	}

	// 2. Check if it's the Alpine Linux download request
	if name == "alpine" {
		url := "https://dl-cdn.alpinelinux.org/alpine/v3.19/releases/x86_64/alpine-virt-3.19.1-x86_64.iso"
		fmt.Printf("Downloading Alpine Linux ISO from %s...\n", url)

		if err := downloadFile(url, diskPath); err != nil {
			return fmt.Errorf("failed to download Alpine ISO: %w", err)
		}

		// Write metadata.json
		meta := ImageMetadata{
			Name:    "alpine",
			Version: "v3.19",
			VMM:     "qemu",
			Source:  url,
		}
		metaData, _ := json.MarshalIndent(meta, "", "  ")
		_ = os.WriteFile(filepath.Join(targetDir, "metadata.json"), metaData, 0644)

		// Write panespec.json
		spec := DefaultProfile()
		spec.VMM = PtrVMMType(VMMQemu)
		spec.CPUs = PtrUint32(1)
		spec.Memory = PtrString("256MiB")
		spec.Disk = &DiskConfig{
			Path:   PtrString(diskPath),
			Format: PtrDiskFormat(FormatRaw),
		}
		spec.Drivers = &DriversConfig{
			VirtioNet: PtrBool(true),
			VirtioBlk: PtrBool(true),
			VirtioRng: PtrBool(false),
		}
		specData, _ := json.MarshalIndent(spec, "", "  ")
		_ = os.WriteFile(filepath.Join(targetDir, "panespec.json"), specData, 0644)

		fmt.Println("alpine pulled and registered successfully!")
		return nil
	}

	// 3. Delegate OCI/Docker container pulls to pane-ctr in Phase 4
	if strings.HasPrefix(ref, "docker://") || strings.HasPrefix(ref, "oci://") || ctrPullFunc != nil {
		if ctrPullFunc != nil {
			fmt.Printf("Delegating container pull for %s to pane-ctr...\n", ref)
			return ctrPullFunc(ref, targetDir)
		}
		return fmt.Errorf("container pull driver (pane-ctr) not registered")
	}

	// 4. Fallback for generic URLs
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		fmt.Printf("Downloading generic ISO from %s...\n", ref)
		if err := downloadFile(ref, diskPath); err != nil {
			return err
		}
		meta := ImageMetadata{
			Name:    name,
			Version: "v1.0",
			VMM:     "qemu",
			Source:  ref,
		}
		metaData, _ := json.MarshalIndent(meta, "", "  ")
		_ = os.WriteFile(filepath.Join(targetDir, "metadata.json"), metaData, 0644)

		spec := DefaultProfile()
		spec.VMM = PtrVMMType(VMMQemu)
		spec.Disk = &DiskConfig{
			Path:   PtrString(diskPath),
			Format: PtrDiskFormat(FormatRaw),
		}
		specData, _ := json.MarshalIndent(spec, "", "  ")
		_ = os.WriteFile(filepath.Join(targetDir, "panespec.json"), specData, 0644)

		return nil
	}

	return fmt.Errorf("unknown image reference scheme: %q", ref)
}

func ListImages() ([]ImageInfo, error) {
	dir := getImagesDir()
	var list []ImageInfo
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		vDir := filepath.Join(dir, name, "v1.0")
		if _, err := os.Stat(vDir); os.IsNotExist(err) {
			subEntries, err := os.ReadDir(filepath.Join(dir, name))
			if err != nil || len(subEntries) == 0 {
				continue
			}
			for _, sub := range subEntries {
				if sub.IsDir() {
					vDir = filepath.Join(dir, name, sub.Name())
					break
				}
			}
		}

		metaPath := filepath.Join(vDir, "metadata.json")
		metaData, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var meta ImageMetadata
		if err := json.Unmarshal(metaData, &meta); err != nil {
			continue
		}

		var size int64
		files, _ := os.ReadDir(vDir)
		for _, f := range files {
			if !f.IsDir() {
				fPath := filepath.Join(vDir, f.Name())
				// Handle symlink
				if info, err := os.Lstat(fPath); err == nil {
					if info.Mode()&os.ModeSymlink != 0 {
						if target, err := os.Readlink(fPath); err == nil {
							if !filepath.IsAbs(target) {
								target = filepath.Join(vDir, target)
							}
							if targetInfo, err := os.Stat(target); err == nil {
								size = targetInfo.Size()
								break
							}
						}
					} else if strings.HasPrefix(f.Name(), "disk.") || strings.HasSuffix(f.Name(), ".iso") || strings.HasSuffix(f.Name(), ".raw") || strings.HasSuffix(f.Name(), ".qcow2") {
						size = info.Size()
						break
					}
				}
			}
		}

		list = append(list, ImageInfo{
			Metadata: meta,
			Size:     size,
		})
	}
	return list, nil
}

func RemoveImage(name string) error {
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, ":", "-")
	dir := getImagesDir()
	target := filepath.Join(dir, name)
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return fmt.Errorf("image %q not found", name)
	}
	return os.RemoveAll(target)
}

func InspectImage(name string) (*PaneSpec, error) {
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, ":", "-")
	dir := getImagesDir()
	vDir := filepath.Join(dir, name, "v1.0")
	specPath := filepath.Join(vDir, "panespec.json")
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		subEntries, err := os.ReadDir(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("image %q not found", name)
		}
		found := false
		for _, sub := range subEntries {
			if sub.IsDir() {
				specPath = filepath.Join(dir, name, sub.Name(), "panespec.json")
				if _, err := os.Stat(specPath); err == nil {
					found = true
					break
				}
			}
		}
		if !found {
			return nil, fmt.Errorf("panespec.json not found for image %q", name)
		}
	}
	return ConfigValidate(specPath)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func downloadFile(url, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
