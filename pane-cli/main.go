package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"pane/pane-api/panespec"
	"pane/pane-api/server"
	"pane/pane-cli/client"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	standaloneFlag     bool
	socketFlag         string
	pullContainerImage func(string, string) error
)

func init() {
	pullContainerImage = panespec.PullContainerImage
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "pane",
		Short: "Pane: Lightweight, embeddable KVM VM lifecycle control plane",
	}

	rootCmd.PersistentFlags().BoolVar(&standaloneFlag, "standalone", false, "Run in standalone mode (direct FFI execution bypassing daemon)")
	rootCmd.PersistentFlags().StringVar(&socketFlag, "socket", "", "Daemon UNIX socket path (defaults to /run/pane.sock or /tmp/pane.sock)")

	// Subcommands
	rootCmd.AddCommand(newDaemonCommand())
	rootCmd.AddCommand(newRunCommand())
	rootCmd.AddCommand(newExecCommand())
	rootCmd.AddCommand(newSnapshotCommand())
	rootCmd.AddCommand(newForkCommand())
	rootCmd.AddCommand(newRmCommand())
	rootCmd.AddCommand(newStopCommand())
	rootCmd.AddCommand(newPsCommand())
	rootCmd.AddCommand(newInspectCommand())
	rootCmd.AddCommand(newLogsCommand())
	rootCmd.AddCommand(newVersionCommand())
	rootCmd.AddCommand(newConfigCommand())
	rootCmd.AddCommand(newImageCommand())
	rootCmd.AddCommand(newPullCommand())
	rootCmd.AddCommand(newImagesCommand())
	rootCmd.AddCommand(newRmiCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// Client helper
func getClient() (client.DaemonClient, func(), error) {
	if standaloneFlag {
		return client.NewEmbeddedClient(), func() {}, nil
	}

	path := socketFlag
	if path == "" {
		path = "/run/pane.sock"
		if _, err := os.Stat(path); os.IsNotExist(err) {
			path = "/tmp/pane.sock"
		}
	}

	conn, err := grpc.Dial("unix://"+path,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(500*time.Millisecond),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to connect to daemon at %s. Falling back to standalone/embedded mode.\n", path)
		return client.NewEmbeddedClient(), func() {}, nil
	}

	cleanup := func() { conn.Close() }
	return client.NewGrpcClient(conn), cleanup, nil
}

// Get standard PID file path
func getPidPath() string {
	if err := os.MkdirAll("/var/lib/pane", 0755); err == nil {
		return "/var/lib/pane/pane.pid"
	}
	return filepath.Join(os.TempDir(), "pane.pid")
}

// Daemon lifecycle management
func newDaemonCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the Pane API daemon",
	}

	var configPath string

	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start the Pane API daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := socketFlag
			if path == "" {
				path = "/run/pane.sock"
				// Fallback if /run is not writable
				if err := os.MkdirAll("/run", 0755); err != nil {
					path = "/tmp/pane.sock"
				}
			}

			// Ensure socket directory exists
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				path = filepath.Join(os.TempDir(), "pane.sock")
			}

			pidPath := getPidPath()
			if err := os.MkdirAll(filepath.Dir(pidPath), 0755); err != nil {
				return err
			}

			// Check if already running
			if data, err := os.ReadFile(pidPath); err == nil {
				if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
					if process, err := os.FindProcess(pid); err == nil {
						if err := process.Signal(syscall.Signal(0)); err == nil {
							return fmt.Errorf("daemon is already running with PID %d", pid)
						}
					}
				}
			}

			fmt.Printf("Starting Pane daemon on UNIX socket: %s\n", path)
			srv, err := server.StartGrpcServerUnix(path)
			if err != nil {
				return fmt.Errorf("failed to start server: %w", err)
			}

			// Write PID file
			pid := os.Getpid()
			if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d\n", pid)), 0644); err != nil {
				srv.GracefulStop()
				return fmt.Errorf("failed to write pid file: %w", err)
			}

			// Wait for signals
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

			for {
				sig := <-sigChan
				if sig == syscall.SIGHUP {
					fmt.Println("Reloading daemon configuration...")
					// In the future, parse config reload here
				} else {
					fmt.Println("Shutting down Pane daemon...")
					srv.GracefulStop()
					_ = os.Remove(path)
					_ = os.Remove(pidPath)
					fmt.Println("Daemon stopped.")
					break
				}
			}
			return nil
		},
	}

	startCmd.Flags().StringVarP(&configPath, "config", "c", "/etc/pane/daemon.yaml", "Path to daemon configuration file")

	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the running Pane API daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			pidPath := getPidPath()
			data, err := os.ReadFile(pidPath)
			if err != nil {
				return fmt.Errorf("daemon is not running (PID file %s not found)", pidPath)
			}

			pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				return fmt.Errorf("invalid PID in pid file: %w", err)
			}

			process, err := os.FindProcess(pid)
			if err != nil {
				return fmt.Errorf("could not find process: %w", err)
			}

			fmt.Printf("Stopping daemon process %d...\n", pid)
			if err := process.Signal(syscall.SIGTERM); err != nil {
				return fmt.Errorf("failed to signal daemon process: %w", err)
			}

			return nil
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Check the status of the Pane API daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			pidPath := getPidPath()
			data, err := os.ReadFile(pidPath)
			if err != nil {
				fmt.Println("Pane daemon is stopped (no PID file).")
				return nil
			}

			pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				return fmt.Errorf("invalid PID in pid file: %w", err)
			}

			process, err := os.FindProcess(pid)
			if err != nil {
				fmt.Printf("Pane daemon is stopped (process %d not found).\n", pid)
				return nil
			}

			if err := process.Signal(syscall.Signal(0)); err != nil {
				fmt.Printf("Pane daemon is stopped (process %d not active).\n", pid)
			} else {
				fmt.Printf("Pane daemon is running (PID: %d).\n", pid)
			}
			return nil
		},
	}

	reloadCmd := &cobra.Command{
		Use:   "reload",
		Short: "Reload the Pane API daemon configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			pidPath := getPidPath()
			data, err := os.ReadFile(pidPath)
			if err != nil {
				return fmt.Errorf("daemon is not running")
			}

			pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				return err
			}

			process, err := os.FindProcess(pid)
			if err != nil {
				return err
			}

			return process.Signal(syscall.SIGHUP)
		},
	}

	cmd.AddCommand(startCmd, stopCmd, statusCmd, reloadCmd)
	return cmd
}

// VM Run Command
func newRunCommand() *cobra.Command {
	var (
		nameFlag      string
		cpusFlag      uint32
		memoryFlag    string
		diskSizeFlag  string
		isoFlag       string
		kernelFlag    string
		cmdlineFlag   string
		virtioNetFlag bool
		noVirtioNet   bool
		virtioBlkFlag bool
		noVirtioBlk   bool
		guiFlag       bool
		displayFlag   string
		configFile    string
		dryRunFlag    bool
	)

	cmd := &cobra.Command{
		Use:   "run [flags] <image>",
		Short: "Run a VM from an image reference or parameters",
		RunE: func(cmd *cobra.Command, args []string) error {
			var image string
			if len(args) > 0 {
				image = args[0]
			}

			// Precedence merging logic
			spec := panespec.DefaultProfile()

			// 1. Overlay image metadata
			if image != "" {
				if specImg, err := panespec.InspectImage(image); err == nil {
					spec = panespec.Merge(spec, specImg)
				} else {
					// Standalone images or remote checks:
					if kernelFlag == "" && isoFlag == "" && !strings.Contains(image, "://") {
						return fmt.Errorf("image %q not found. Please run \"pane pull %s\" first", image, image)
					}
				}
			}

			// 2. Overlay config file (-f / --config)
			if configFile != "" {
				fileSpec, err := panespec.ConfigValidate(configFile)
				if err != nil {
					return fmt.Errorf("failed to parse config file: %w", err)
				}
				spec = panespec.Merge(spec, fileSpec)
			}

			// 3. Overlay CLI flags
			cliSpec := PaneSpecToPaneSpec(
				nameFlag, cpusFlag, memoryFlag, diskSizeFlag, isoFlag, kernelFlag,
				cmdlineFlag, virtioNetFlag, noVirtioNet, virtioBlkFlag, noVirtioBlk,
				guiFlag, displayFlag, cmd.Flags().Args(),
			)
			spec = panespec.Merge(spec, cliSpec)

			// Validate merged spec
			if err := panespec.Validate(spec); err != nil {
				return fmt.Errorf("validation error: %w", err)
			}

			if dryRunFlag {
				fmt.Println("Merged spec configuration:")
				// In Phase 2, this will print QEMU argv representation
				fmt.Printf("VMM Backend: %v\n", getStrValGeneric(spec.VMM))
				fmt.Printf("vCPUs: %v\n", getUintVal(spec.CPUs))
				fmt.Printf("Memory: %v\n", getStrVal(spec.Memory))
				if spec.Disk != nil {
					fmt.Printf("Disk: Path=%v, Size=%v, Format=%v\n", getStrVal(spec.Disk.Path), getStrVal(spec.Disk.Size), getStrValGeneric(spec.Disk.Format))
				}
				fmt.Printf("Kernel Path: %v\n", getStrVal(spec.Kernel))
				fmt.Printf("Cmdline: %v\n", getStrVal(spec.Cmdline))
				fmt.Printf("Extra Args: %v\n", spec.ExtraArgs)
				return nil
			}

			vmID := nameFlag
			if vmID == "" {
				vmID = fmt.Sprintf("vm-%d", time.Now().UnixNano()%1000000)
			}

			client, cleanup, err := getClient()
			if err != nil {
				return err
			}
			defer cleanup()

			ctx := context.Background()
			cid, pid, err := client.Spawn(ctx, vmID, spec)
			if err != nil {
				return fmt.Errorf("failed to spawn VM: %w", err)
			}

			fmt.Printf("VM %s booted successfully!\nPID: %d\nVsock CID: %d\n", vmID, pid, cid)
			return nil
		},
	}

	cmd.Flags().StringVar(&nameFlag, "name", "", "VM name")
	cmd.Flags().Uint32Var(&cpusFlag, "cpus", 0, "Number of vCPUs")
	cmd.Flags().StringVar(&memoryFlag, "memory", "", "Memory size (e.g. 512MiB, 2GiB)")
	cmd.Flags().StringVar(&diskSizeFlag, "disk", "", "Disk size")
	cmd.Flags().StringVar(&isoFlag, "iso", "", "ISO image path (overrides disk path)")
	cmd.Flags().StringVar(&kernelFlag, "kernel", "", "Kernel path")
	cmd.Flags().StringVar(&cmdlineFlag, "cmdline", "", "Kernel command line")
	cmd.Flags().BoolVar(&virtioNetFlag, "virtio-net", false, "Enable virtio-net")
	cmd.Flags().BoolVar(&noVirtioNet, "no-virtio-net", false, "Disable virtio-net")
	cmd.Flags().BoolVar(&virtioBlkFlag, "virtio-blk", false, "Enable virtio-blk")
	cmd.Flags().BoolVar(&noVirtioBlk, "no-virtio-blk", false, "Disable virtio-blk")
	cmd.Flags().BoolVar(&guiFlag, "gui", false, "Enable GUI display")
	cmd.Flags().StringVar(&displayFlag, "display", "", "GUI display type (e.g., gtk, sdl)")
	cmd.Flags().StringVarP(&configFile, "config", "f", "", "Load configuration from file")
	cmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "Print spec and exit without executing VM")

	return cmd
}

// Convert flags to partial spec
func PaneSpecToPaneSpec(
	name string, cpus uint32, memory, disk, iso, kernel, cmdline string,
	vnet, novnet, vblk, novblk, gui bool, display string, extraArgs []string,
) *panespec.PaneSpec {
	spec := &panespec.PaneSpec{}
	if cpus > 0 {
		spec.CPUs = panespec.PtrUint32(cpus)
	}
	if memory != "" {
		spec.Memory = panespec.PtrString(memory)
	}
	if disk != "" || iso != "" {
		spec.Disk = &panespec.DiskConfig{}
		if iso != "" {
			spec.Disk.Path = panespec.PtrString(iso)
		} else if disk != "" {
			spec.Disk.Path = panespec.PtrString(disk)
		}
	}
	if kernel != "" {
		spec.Kernel = panespec.PtrString(kernel)
	}
	if cmdline != "" {
		spec.Cmdline = panespec.PtrString(cmdline)
	}

	if vnet || novnet || vblk || novblk {
		spec.Drivers = &panespec.DriversConfig{}
		if vnet {
			spec.Drivers.VirtioNet = panespec.PtrBool(true)
		} else if novnet {
			spec.Drivers.VirtioNet = panespec.PtrBool(false)
		}
		if vblk {
			spec.Drivers.VirtioBlk = panespec.PtrBool(true)
		} else if novblk {
			spec.Drivers.VirtioBlk = panespec.PtrBool(false)
		}
	}

	if gui || display != "" {
		mode := "gtk"
		if display != "" {
			mode = display
		}
		spec.ExtraArgs = append(spec.ExtraArgs, "-display", mode)
	}

	if len(extraArgs) > 0 {
		spec.ExtraArgs = append(spec.ExtraArgs, extraArgs...)
	}

	return spec
}

func getStrVal(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func getUintVal(ptr *uint32) uint32 {
	if ptr == nil {
		return 0
	}
	return *ptr
}

func getStrValGeneric[T ~string](ptr *T) string {
	if ptr == nil {
		return ""
	}
	return string(*ptr)
}

// VM Exec Command
func newExecCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec <vm> -- <cmd...>",
		Short: "Execute a command inside a running VM",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			vmID := args[0]
			command := args[1]
			cmdArgs := args[2:]

			client, cleanup, err := getClient()
			if err != nil {
				return err
			}
			defer cleanup()

			ctx := context.Background()
			cb := func(data []byte, isStderr bool, exitCode int32) {
				if len(data) > 0 {
					if isStderr {
						os.Stderr.Write(data)
					} else {
						os.Stdout.Write(data)
					}
				}
				if exitCode >= 0 {
					os.Exit(int(exitCode))
				}
			}

			err = client.Exec(ctx, vmID, command, cmdArgs, cb)
			if err != nil {
				return fmt.Errorf("execution error: %w", err)
			}
			return nil
		},
	}
	return cmd
}

// Snapshot Command
func newSnapshotCommand() *cobra.Command {
	var tagFlag string
	cmd := &cobra.Command{
		Use:   "snapshot <vm>",
		Short: "Pause and snapshot VM state to file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vmID := args[0]

			tag := tagFlag
			if tag == "" {
				tag = "latest"
			}

			snapshotDir := "/var/lib/pane/snapshots"
			if err := os.MkdirAll(snapshotDir, 0755); err != nil {
				snapshotDir = os.TempDir()
			}

			snapPath := filepath.Join(snapshotDir, fmt.Sprintf("%s_%s.snap", vmID, tag))
			memPath := filepath.Join(snapshotDir, fmt.Sprintf("%s_%s.mem", vmID, tag))

			client, cleanup, err := getClient()
			if err != nil {
				return err
			}
			defer cleanup()

			ctx := context.Background()
			fmt.Printf("Taking snapshot of VM %s (tag: %s)...\n", vmID, tag)
			err = client.Snapshot(ctx, vmID, snapPath, memPath)
			if err != nil {
				return fmt.Errorf("snapshot failed: %w", err)
			}

			fmt.Printf("Snapshot completed successfully!\nSnap file: %s\nMemory file: %s\n", snapPath, memPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&tagFlag, "tag", "latest", "Snapshot tag name")
	return cmd
}

// Fork Command
func newForkCommand() *cobra.Command {
	var (
		countFlag  int
		prefixFlag string
		tagFlag    string
	)

	cmd := &cobra.Command{
		Use:   "fork <vm>",
		Short: "Instantly clone/fork new VM instances from a snapshot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vmID := args[0]

			tag := tagFlag
			if tag == "" {
				tag = "latest"
			}

			snapshotDir := "/var/lib/pane/snapshots"
			snapPath := filepath.Join(snapshotDir, fmt.Sprintf("%s_%s.snap", vmID, tag))
			memPath := filepath.Join(snapshotDir, fmt.Sprintf("%s_%s.mem", vmID, tag))

			if _, err := os.Stat(snapPath); os.IsNotExist(err) {
				// Fallback to temp dir
				snapshotDir = os.TempDir()
				snapPath = filepath.Join(snapshotDir, fmt.Sprintf("%s_%s.snap", vmID, tag))
				memPath = filepath.Join(snapshotDir, fmt.Sprintf("%s_%s.mem", vmID, tag))
				if _, err := os.Stat(snapPath); os.IsNotExist(err) {
					return fmt.Errorf("snapshot file not found: %s", snapPath)
				}
			}

			client, cleanup, err := getClient()
			if err != nil {
				return err
			}
			defer cleanup()

			ctx := context.Background()
			prefix := prefixFlag
			if prefix == "" {
				prefix = fmt.Sprintf("%s-fork", vmID)
			}

			for i := 1; i <= countFlag; i++ {
				forkID := fmt.Sprintf("%s-%d", prefix, i)
				newRootfs := fmt.Sprintf("/var/lib/pane/instances/%s/rootfs.img", forkID)
				_ = os.MkdirAll(filepath.Dir(newRootfs), 0755)

				// Generate unique CID (>= 3)
				newCid := uint32(10 + i)

				fmt.Printf("Forking clone %d of %d: ID=%s, CID=%d...\n", i, countFlag, forkID, newCid)
				pid, err := client.Fork(ctx, forkID, snapPath, memPath, newRootfs, newCid)
				if err != nil {
					return fmt.Errorf("failed to fork clone %s: %w", forkID, err)
				}
				fmt.Printf("Clone %s forked successfully! PID: %d\n", forkID, pid)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&countFlag, "count", 1, "Number of forks to spawn")
	cmd.Flags().StringVar(&prefixFlag, "prefix", "", "Prefix name for the forks")
	cmd.Flags().StringVar(&tagFlag, "tag", "latest", "Source snapshot tag name")
	return cmd
}

// VM Rm/Destroy Command
func newRmCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm <vm>",
		Short: "Reclaim resources and destroy a VM",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vmID := args[0]

			client, cleanup, err := getClient()
			if err != nil {
				return err
			}
			defer cleanup()

			ctx := context.Background()
			fmt.Printf("Destroying VM %s...\n", vmID)
			err = client.Destroy(ctx, vmID)
			if err != nil {
				return fmt.Errorf("failed to destroy VM: %w", err)
			}

			fmt.Printf("VM %s destroyed successfully.\n", vmID)
			return nil
		},
	}
	return cmd
}

// VM Stop (Stub, maps to destroy)
func newStopCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop <vm>",
		Short: "Stop a running VM (equivalent to rm/destroy in this version)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vmID := args[0]

			client, cleanup, err := getClient()
			if err != nil {
				return err
			}
			defer cleanup()

			ctx := context.Background()
			fmt.Printf("Stopping VM %s...\n", vmID)
			err = client.Destroy(ctx, vmID)
			if err != nil {
				return fmt.Errorf("failed to stop VM: %w", err)
			}

			fmt.Printf("VM %s stopped.\n", vmID)
			return nil
		},
	}
	return cmd
}

// VM List/PS Command
func newPsCommand() *cobra.Command {
	var allFlag bool
	cmd := &cobra.Command{
		Use:   "ps",
		Short: "List VM instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			dirs := []string{"/run/pane", "/tmp/pane"}
			fmt.Printf("%-20s %-10s %-15s\n", "VM ID", "STATUS", "SOCKET")

			found := false
			for _, d := range dirs {
				files, err := os.ReadDir(d)
				if err != nil {
					continue
				}
				for _, f := range files {
					if !f.IsDir() && strings.HasPrefix(f.Name(), "fc-") && strings.HasSuffix(f.Name(), ".sock") {
						// Extract VM ID
						vmID := strings.TrimPrefix(f.Name(), "fc-")
						vmID = strings.TrimSuffix(vmID, ".sock")
						// Skip vsock sockets
						if strings.HasPrefix(vmID, "vsock-") {
							continue
						}
						found = true
						socketPath := filepath.Join(d, f.Name())
						fmt.Printf("%-20s %-10s %-15s\n", vmID, "RUNNING", socketPath)
					}
				}
			}

			if !found {
				fmt.Println("No running VMs found.")
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&allFlag, "all", "a", false, "Show all VMs (running and dead)")
	return cmd
}

// VM Inspect Command (Stub)
func newInspectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect <vm>",
		Short: "Display detailed information on a VM",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			vmID := args[0]
			fmt.Printf("VM Details for %s:\n", vmID)
			fmt.Println("Status: RUNNING")
			fmt.Println("Backend: firecracker")
		},
	}
	return cmd
}

// VM Logs Command (Stub)
func newLogsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs <vm>",
		Short: "Fetch log output of a VM",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vmID := args[0]
			logPath := fmt.Sprintf("/run/pane/fc-%s.log", vmID)
			if _, err := os.Stat(logPath); os.IsNotExist(err) {
				logPath = fmt.Sprintf("/tmp/pane/fc-%s.log", vmID)
				if _, err := os.Stat(logPath); os.IsNotExist(err) {
					return fmt.Errorf("no logs found for VM %s", vmID)
				}
			}

			file, err := os.Open(logPath)
			if err != nil {
				return err
			}
			defer file.Close()

			_, _ = io.Copy(os.Stdout, file)
			return nil
		},
	}
	return cmd
}

// Config Subcommands (Init and Validate)
func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration profiles",
	}

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold a commented template pane.yaml file",
		RunE: func(cmd *cobra.Command, args []string) error {
			content := panespec.ConfigInit()
			fmt.Println(content)
			return nil
		},
	}

	validateCmd := &cobra.Command{
		Use:   "validate <file>",
		Short: "Validate a configuration file (YAML/JSON) against the schema",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			file := args[0]
			spec, err := panespec.ConfigValidate(file)
			if err != nil {
				return err
			}
			fmt.Printf("Configuration is valid! Merged schema details:\n")
			fmt.Printf("  VMM Type: %v\n", getStrValGeneric(spec.VMM))
			fmt.Printf("  vCPUs: %v\n", getUintVal(spec.CPUs))
			fmt.Printf("  Memory: %v\n", getStrVal(spec.Memory))
			if spec.Disk != nil {
				fmt.Printf("  Disk format: %v\n", getStrValGeneric(spec.Disk.Format))
				fmt.Printf("  Disk size: %v\n", getStrVal(spec.Disk.Size))
				fmt.Printf("  Disk path: %v\n", getStrVal(spec.Disk.Path))
			}
			return nil
		},
	}

	cmd.AddCommand(initCmd, validateCmd)
	return cmd
}

// Image commands
func newPullCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "pull <ref>",
		Short: "Pull an image (manifest, container, URL, or local path)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Pulling image: %s...\n", args[0])
			return panespec.PullImage(args[0], pullContainerImage)
		},
	}
}

func newImagesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "images",
		Short: "List cached images",
		RunE: func(cmd *cobra.Command, args []string) error {
			list, err := panespec.ListImages()
			if err != nil {
				return err
			}
			if len(list) == 0 {
				fmt.Println("No cached images.")
				return nil
			}
			fmt.Printf("%-30s %-10s %-15s %-12s\n", "IMAGE NAME", "VERSION", "VMM BACKEND", "SIZE")
			for _, img := range list {
				sizeStr := formatBytes(img.Size)
				fmt.Printf("%-30s %-10s %-15s %-12s\n", img.Metadata.Name, img.Metadata.Version, img.Metadata.VMM, sizeStr)
			}
			return nil
		},
	}
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
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func newRmiCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "rmi <image>",
		Short: "Remove a cached image",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Removing image %s...\n", args[0])
			err := panespec.RemoveImage(args[0])
			if err != nil {
				return err
			}
			fmt.Println("Image removed successfully.")
			return nil
		},
	}
}

func newImageInspectCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <image>",
		Short: "Display resolved panespec for the image",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := panespec.InspectImage(args[0])
			if err != nil {
				return err
			}
			data, _ := json.MarshalIndent(spec, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}
}

func newImageCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: "Manage images",
	}

	cmd.AddCommand(newPullCommand(), newImagesCommand(), newRmiCommand(), newImageInspectCommand())
	return cmd
}

// Version Command
func newVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version information of Pane components",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Pane CLI Version: v0.2.0")
			fmt.Println("Pane API Server Version: v0.2.0")
			fmt.Println("Pane Core Library Version: v0.2.0")
		},
	}
	return cmd
}
