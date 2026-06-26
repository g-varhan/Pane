// SPDX-License-Identifier: Apache-2.0

package panespec

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ────────────────────────────────────────────────────────────────────────────
// Default download URLs for proprietary / external images
// ────────────────────────────────────────────────────────────────────────────

const (
	// windows – tiny10 (minimal Windows 11 23H2) image hosted on the Internet Archive.
	// Direct link; archive.org redirects to a CDN node (302 → HTTPS).
	defaultWindowsURL = "https://archive.org/download/tiny-10-23-h2/tiny10%20x64%2023h2.iso"

	// VirtIO-Win – Fedora-hosted driver ISO for Windows guests.
	// Corrected filename (the .iso symlink redirects to the versioned file).
	defaultVirtioWinURL = "https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/archive-virtio/virtio-win-0.1.285-1/virtio-win-0.1.285.iso"
)

// ────────────────────────────────────────────────────────────────────────────
// Image metadata types
// ────────────────────────────────────────────────────────────────────────────

type ImageMetadata struct {
	Name            string   `json:"name"`
	Version         string   `json:"version"`
	VMM             string   `json:"vmm"`
	Source          string   `json:"source"`
	KernelPath      string   `json:"kernel_path,omitempty"`
	DriversRequired []string `json:"drivers_required,omitempty"`
}

type ImageInfo struct {
	Metadata ImageMetadata `json:"metadata"`
	Size     int64         `json:"size"`
}

// ────────────────────────────────────────────────────────────────────────────
// Built-in distribution registry
// ────────────────────────────────────────────────────────────────────────────

type distroEntry struct {
	version string
	url     string
	cpus    uint32
	memory  string
}

// knownDistros maps a short name to its download URL and default resources.
// URLs always point to the latest stable release available via a stable mirror.
var knownDistros = map[string]distroEntry{
	// ── Alpine ──────────────────────────────────────────────────────────────
	"alpine": {
		version: "v3.21",
		url:     "https://dl-cdn.alpinelinux.org/alpine/v3.21/releases/x86_64/alpine-virt-3.21.0-x86_64.iso",
		cpus:    1,
		memory:  "512MiB",
	},
	// ── Ubuntu ──────────────────────────────────────────────────────────────
	"ubuntu": {
		version: "v24.04",
		url:     "https://releases.ubuntu.com/24.04/ubuntu-24.04.2-live-server-amd64.iso",
		cpus:    2,
		memory:  "2GiB",
	},
	"ubuntu-desktop": {
		version: "v26.04",
		url:     "https://releases.ubuntu.com/26.04/ubuntu-26.04-desktop-amd64.iso",
		cpus:    2,
		memory:  "4GiB",
	},
	"ubuntu-minimal": {
		version: "v24.04",
		url:     "https://releases.ubuntu.com/24.04/ubuntu-24.04.2-live-server-amd64.iso",
		cpus:    1,
		memory:  "1GiB",
	},
	// ── Debian ──────────────────────────────────────────────────────────────
	"debian": {
		version: "v12",
		url:     "https://cdimage.debian.org/debian-cd/current/amd64/iso-cd/debian-12.10.0-amd64-netinst.iso",
		cpus:    1,
		memory:  "1GiB",
	},
	"debian-live": {
		version: "v12",
		url:     "https://cdimage.debian.org/debian-cd/current-live/amd64/iso-hybrid/debian-live-12.10.0-amd64-gnome.iso",
		cpus:    2,
		memory:  "2GiB",
	},
	// ── Fedora ──────────────────────────────────────────────────────────────
	"fedora": {
		version: "v41",
		url:     "https://download.fedoraproject.org/pub/fedora/linux/releases/41/Server/x86_64/iso/Fedora-Server-dvd-x86_64-41-1.4.iso",
		cpus:    2,
		memory:  "2GiB",
	},
	"fedora-workstation": {
		version: "v41",
		url:     "https://download.fedoraproject.org/pub/fedora/linux/releases/41/Workstation/x86_64/iso/Fedora-Workstation-Live-x86_64-41-1.4.iso",
		cpus:    2,
		memory:  "4GiB",
	},
	// ── Arch Linux ──────────────────────────────────────────────────────────
	"arch": {
		version: "latest",
		url:     "https://geo.mirror.pkgbuild.com/iso/latest/archlinux-x86_64.iso",
		cpus:    1,
		memory:  "1GiB",
	},
	// ── Kali Linux ──────────────────────────────────────────────────────────
	"kali": {
		version: "v2024.4",
		url:     "https://cdimage.kali.org/kali-2024.4/kali-linux-2024.4-installer-amd64.iso",
		cpus:    2,
		memory:  "2GiB",
	},
	// ── openSUSE ────────────────────────────────────────────────────────────
	"opensuse": {
		version: "v15.6",
		url:     "https://download.opensuse.org/distribution/leap/15.6/iso/openSUSE-Leap-15.6-DVD-x86_64-Media.iso",
		cpus:    2,
		memory:  "2GiB",
	},
	// ── Rocky Linux ─────────────────────────────────────────────────────────
	"rocky": {
		version: "v9.4",
		url:     "https://download.rockylinux.org/pub/rocky/9/isos/x86_64/Rocky-9.4-x86_64-minimal.iso",
		cpus:    2,
		memory:  "2GiB",
	},
	// ── AlmaLinux ───────────────────────────────────────────────────────────
	"alma": {
		version: "v9.4",
		url:     "https://repo.almalinux.org/almalinux/9/isos/x86_64/AlmaLinux-9.4-x86_64-minimal.iso",
		cpus:    2,
		memory:  "2GiB",
	},
}

// ────────────────────────────────────────────────────────────────────────────
// Helpers
// ────────────────────────────────────────────────────────────────────────────

func getImagesDir() string {
	dir := "/var/lib/pane/images"
	if err := os.MkdirAll(dir, 0755); err != nil {
		dir = filepath.Join(os.TempDir(), "pane/images")
		_ = os.MkdirAll(dir, 0755)
	}
	return dir
}

// ────────────────────────────────────────────────────────────────────────────
// PullImage
// ────────────────────────────────────────────────────────────────────────────

// PullImage resolves the ref (built-in distro, local file, GitHub release, or
// HTTP/HTTPS URL) and registers the image in /var/lib/pane/images.
func PullImage(ref string, ctrPullFunc func(string, string) error) error {
	name := strings.TrimPrefix(ref, "pane://")
	name = strings.TrimPrefix(name, "docker://")
	name = strings.TrimPrefix(name, "oci://")
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, ":", "-")

	// 🛡️ Security: Prevent path traversal and manipulation using special directory names
	if name == "" || name == "." || name == ".." {
		return fmt.Errorf("invalid image name %q", name)
	}

	imagesDir := getImagesDir()
	targetDir := filepath.Join(imagesDir, name, "v1.0")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create image dir: %w", err)
	}
	diskPath := filepath.Join(targetDir, "disk.iso")

	// ── 1. Windows (tiny10 – testing only) ──────────────────────────────────
	if name == "windows" {
		return pullWindows(diskPath, targetDir)
	}

	// ── 2. Known Linux distros ──────────────────────────────────────────────
	if entry, ok := knownDistros[name]; ok {
		return pullDistro(name, entry, diskPath, targetDir)
	}

	// ── 3. OCI/Docker containers ────────────────────────────────────────────
	if strings.HasPrefix(ref, "docker://") || strings.HasPrefix(ref, "oci://") {
		if ctrPullFunc != nil {
			fmt.Printf("Delegating container pull for %s to pane-ctr...\n", ref)
			return ctrPullFunc(ref, targetDir)
		}
		return fmt.Errorf("container pull driver (pane-ctr) not registered")
	}

	// ── 4. Generic HTTP(S) URL ──────────────────────────────────────────────
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		fmt.Printf("Downloading ISO from %s\n", ref)
		if err := downloadWithProgress(ref, diskPath); err != nil {
			return err
		}
		writeGenericMeta(name, ref, diskPath, targetDir)
		return nil
	}

	return fmt.Errorf("unknown image reference %q\n\nAvailable built-in distros: %s", ref, listKnownDistros())
}

// ────────────────────────────────────────────────────────────────────────────
// windows pull logic
// ────────────────────────────────────────────────────────────────────────────

func pullWindows(diskPath, targetDir string) error {
	// ── Resolve Windows ISO source (priority: env override > web) ───────────
	windowsURL := os.Getenv("GITHUB_WINDOWS_URL")
	if windowsURL == "" {
		windowsURL = os.Getenv("WINDOWS_URL")
	}

	if windowsURL != "" || true {
		// Check local path first so we never re-download unnecessarily.
		srcIso := os.Getenv("WINDOWS_ISO_PATH")
		if srcIso == "" {
			srcIso = "/var/lib/pane/images/tiny10/v1.0/disk.iso"
		}

		if _, err := os.Stat(srcIso); err == nil {
			// Local ISO exists — register via symlink.
			fmt.Printf("  Found local Windows ISO (tiny10): %s\n", srcIso)
			_ = os.Remove(diskPath)
			if err := os.Symlink(srcIso, diskPath); err != nil {
				if err := copyFile(srcIso, diskPath); err != nil {
					return fmt.Errorf("failed to link/copy Windows ISO: %w", err)
				}
			}
		} else {
			// Download from web.
			url := windowsURL
			if url == "" {
				url = defaultWindowsURL
			}
			fmt.Printf("\n  Pulling Windows (tiny10 – testing purposes only)\n")
			fmt.Printf("  Source : %s\n\n", url)
			if err := downloadWithProgress(url, diskPath); err != nil {
				return fmt.Errorf("failed to download Windows ISO: %w", err)
			}
		}
	}

	// ── Resolve VirtIO-Win driver ISO ────────────────────────────────────────
	virtioPath := resolveVirtioWin(targetDir)

	writeWindowsMeta(diskPath, virtioPath, targetDir)
	fmt.Println("\n✓  Windows (tiny10) registered successfully!")
	return nil
}

// resolveVirtioWin returns the local path to the VirtIO-Win driver ISO,
// downloading it if necessary.
func resolveVirtioWin(targetDir string) string {
	// 1. Explicit env override.
	if p := os.Getenv("VIRTIO_WIN_ISO"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// 2. Default local path (legacy).
	legacy := "/var/lib/pane/images/virtio-win.iso"
	if _, err := os.Stat(legacy); err == nil {
		return legacy
	}

	// 3. Already downloaded alongside the tiny10 image.
	cached := filepath.Join(targetDir, "virtio-win.iso")
	if _, err := os.Stat(cached); err == nil {
		return cached
	}

	// 4. Download from Fedora CDN.
	virtioURL := os.Getenv("VIRTIO_WIN_URL")
	if virtioURL == "" {
		virtioURL = defaultVirtioWinURL
	}
	fmt.Printf("\n  Pulling VirtIO-Win drivers\n")
	fmt.Printf("  Source : %s\n\n", virtioURL)
	if err := downloadWithProgress(virtioURL, cached); err != nil {
		fmt.Printf("  ⚠  VirtIO-Win download failed (%v) — Windows VM will boot without drivers\n", err)
		return ""
	}
	fmt.Println("  ✓  VirtIO-Win drivers cached at", cached)
	return cached
}

// writeWindowsMeta writes metadata.json and panespec.json for a Windows (tiny10) image.
// virtioPath may be empty if the driver ISO is unavailable.
func writeWindowsMeta(diskPath, virtioPath, targetDir string) {
	source := defaultWindowsURL
	if p := os.Getenv("WINDOWS_ISO_PATH"); p != "" {
		source = "local://" + p
	}
	if u := os.Getenv("GITHUB_WINDOWS_URL"); u != "" {
		source = u
	}
	meta := ImageMetadata{Name: "windows", Version: "23H2", VMM: "qemu", Source: source}
	writeMetaJSON(targetDir, meta)

	spec := DefaultProfile()
	spec.VMM = PtrVMMType(VMMQemu)
	spec.CPUs = PtrUint32(4)
	spec.Memory = PtrString("4GiB")
	spec.Disk = &DiskConfig{Path: PtrString(diskPath), Format: PtrDiskFormat(FormatRaw)}
	spec.Drivers = &DriversConfig{VirtioNet: PtrBool(true), VirtioBlk: PtrBool(true), VirtioRng: PtrBool(false)}
	if virtioPath != "" {
		spec.ExtraArgs = []string{"-cdrom", virtioPath, "-vnc", ":1"}
	} else {
		spec.ExtraArgs = []string{"-vnc", ":1"}
	}
	writeSpecJSON(targetDir, spec)
}

// ────────────────────────────────────────────────────────────────────────────
// Linux distro pull logic
// ────────────────────────────────────────────────────────────────────────────

func pullDistro(name string, entry distroEntry, diskPath, targetDir string) error {
	printBanner(name, entry)

	if err := downloadWithProgress(entry.url, diskPath); err != nil {
		return fmt.Errorf("failed to download %s ISO: %w", name, err)
	}

	meta := ImageMetadata{Name: name, Version: entry.version, VMM: "qemu", Source: entry.url}
	writeMetaJSON(targetDir, meta)

	spec := DefaultProfile()
	spec.VMM = PtrVMMType(VMMQemu)
	spec.CPUs = PtrUint32(entry.cpus)
	spec.Memory = PtrString(entry.memory)
	spec.Disk = &DiskConfig{Path: PtrString(diskPath), Format: PtrDiskFormat(FormatRaw)}
	spec.Drivers = &DriversConfig{VirtioNet: PtrBool(true), VirtioBlk: PtrBool(true), VirtioRng: PtrBool(false)}
	writeSpecJSON(targetDir, spec)

	fmt.Printf("\n✓  %s (%s) pulled and registered successfully!\n", name, entry.version)
	return nil
}

// ────────────────────────────────────────────────────────────────────────────
// Pretty download progress bar (no extra deps)
// ────────────────────────────────────────────────────────────────────────────

// progressWriter wraps an io.Writer and prints a live terminal progress bar.
type progressWriter struct {
	total     int64
	written   int64
	lastPrint time.Time
	barWidth  int
	startTime time.Time
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.written += int64(n)
	now := time.Now()
	if now.Sub(pw.lastPrint) < 100*time.Millisecond && pw.written < pw.total {
		return n, nil
	}
	pw.lastPrint = now
	pw.printBar()
	return n, nil
}

func (pw *progressWriter) printBar() {
	elapsed := time.Since(pw.startTime).Seconds()
	speed := float64(pw.written) / elapsed // bytes/s
	speedStr := formatBytes(int64(speed)) + "/s"
	doneStr := formatBytes(pw.written)
	totalStr := "?"
	if pw.total > 0 {
		totalStr = formatBytes(pw.total)
	}

	var bar string
	if pw.total > 0 {
		pct := float64(pw.written) / float64(pw.total)
		filled := int(pct * float64(pw.barWidth))
		bar = "[" + strings.Repeat("█", filled) + strings.Repeat("░", pw.barWidth-filled) + "]"
		fmt.Printf("\r  %s %3.0f%%  %s / %s  %s    ", bar, pct*100, doneStr, totalStr, speedStr)
	} else {
		fmt.Printf("\r  Downloading...  %s  %s    ", doneStr, speedStr)
	}
}

func (pw *progressWriter) finish() {
	pw.written = pw.total
	pw.printBar()
	fmt.Println()
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// httpClient is configured to:
//   - Follow HTTP → HTTPS redirects (Go's default client blocks these by default
//     when the scheme downgrades, but upgrades are always followed).
//   - Send a browser-like User-Agent so servers like archive.org and
//     fedorapeople.org serve the file instead of returning 403.
var httpClient = &http.Client{
	Timeout: 0, // no overall timeout — files can be many GiB
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		// Allow up to 15 redirects (archive.org uses 302 → CDN node).
		if len(via) >= 15 {
			return fmt.Errorf("too many redirects")
		}
		// Carry the User-Agent through all hops.
		req.Header.Set("User-Agent", browserUA)
		return nil
	},
}

const browserUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 " +
	"(KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"

func downloadWithProgress(url, destPath string) error {
	fmt.Printf("  Connecting to %s\n", url)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("bad URL: %w", err)
	}
	// Mimic a real browser enough to pass basic bot-checks.
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned HTTP %d for %s", resp.StatusCode, url)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("cannot create destination file: %w", err)
	}
	defer out.Close()

	pw := &progressWriter{
		total:     resp.ContentLength,
		barWidth:  30,
		startTime: time.Now(),
		lastPrint: time.Now(),
	}

	reader := io.TeeReader(resp.Body, pw)
	if _, err := io.Copy(out, reader); err != nil {
		// Remove partial file so a retry starts clean.
		_ = out.Close()
		_ = os.Remove(destPath)
		return fmt.Errorf("download interrupted: %w", err)
	}
	pw.finish()
	return nil
}

// ────────────────────────────────────────────────────────────────────────────
// Generic metadata helpers
// ────────────────────────────────────────────────────────────────────────────

func writeMetaJSON(targetDir string, meta ImageMetadata) {
	data, _ := json.MarshalIndent(meta, "", "  ")
	_ = os.WriteFile(filepath.Join(targetDir, "metadata.json"), data, 0644)
}

func writeSpecJSON(targetDir string, spec *PaneSpec) {
	data, _ := json.MarshalIndent(spec, "", "  ")
	_ = os.WriteFile(filepath.Join(targetDir, "panespec.json"), data, 0644)
}

func writeGenericMeta(name, source, diskPath, targetDir string) {
	meta := ImageMetadata{Name: name, Version: "v1.0", VMM: "qemu", Source: source}
	writeMetaJSON(targetDir, meta)
	spec := DefaultProfile()
	spec.VMM = PtrVMMType(VMMQemu)
	spec.Disk = &DiskConfig{Path: PtrString(diskPath), Format: PtrDiskFormat(FormatRaw)}
	writeSpecJSON(targetDir, spec)
}

func printBanner(name string, entry distroEntry) {
	fmt.Printf("\n  Pulling %s %s\n", name, entry.version)
	fmt.Printf("  Source : %s\n", entry.url)
	fmt.Printf("  CPUs   : %d    Memory: %s\n\n", entry.cpus, entry.memory)
}

func listKnownDistros() string {
	names := make([]string, 0, len(knownDistros))
	for k := range knownDistros {
		names = append(names, k)
	}
	// Sort for deterministic output.
	for i := 0; i < len(names)-1; i++ {
		for j := i + 1; j < len(names); j++ {
			if names[i] > names[j] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	return strings.Join(names, ", ")
}

// ────────────────────────────────────────────────────────────────────────────
// ListImages / RemoveImage / InspectImage
// ────────────────────────────────────────────────────────────────────────────

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
		imgName := entry.Name()
		vDir := filepath.Join(dir, imgName, "v1.0")
		if _, err := os.Stat(vDir); os.IsNotExist(err) {
			subEntries, err := os.ReadDir(filepath.Join(dir, imgName))
			if err != nil || len(subEntries) == 0 {
				continue
			}
			for _, sub := range subEntries {
				if sub.IsDir() {
					vDir = filepath.Join(dir, imgName, sub.Name())
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
			if f.IsDir() {
				continue
			}
			fPath := filepath.Join(vDir, f.Name())
			info, err := os.Lstat(fPath)
			if err != nil {
				continue
			}
			if info.Mode()&os.ModeSymlink != 0 {
				target, err := os.Readlink(fPath)
				if err != nil {
					continue
				}
				if !filepath.IsAbs(target) {
					target = filepath.Join(vDir, target)
				}
				if targetInfo, err := os.Stat(target); err == nil {
					size = targetInfo.Size()
					break
				}
			} else if strings.HasPrefix(f.Name(), "disk.") ||
				strings.HasSuffix(f.Name(), ".iso") ||
				strings.HasSuffix(f.Name(), ".raw") ||
				strings.HasSuffix(f.Name(), ".qcow2") {
				size = info.Size()
				break
			}
		}

		list = append(list, ImageInfo{Metadata: meta, Size: size})
	}
	return list, nil
}

func RemoveImage(name string) error {
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, ":", "-")

	// 🛡️ Security: Prevent path traversal to avoid arbitrary directory deletion
	if name == "" || name == "." || name == ".." {
		return fmt.Errorf("invalid image name %q", name)
	}

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

	// 🛡️ Security: Prevent path traversal and arbitrary filesystem inspection
	if name == "" || name == "." || name == ".." {
		return nil, fmt.Errorf("invalid image name %q", name)
	}

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

// ────────────────────────────────────────────────────────────────────────────
// Low-level file helpers
// ────────────────────────────────────────────────────────────────────────────

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

// downloadFile is kept for internal backwards-compat (no progress bar).
func downloadFile(url, filePath string) error {
	return downloadWithProgress(url, filePath)
}
