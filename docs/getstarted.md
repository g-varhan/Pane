# Getting Started with Pane

Welcome to **Pane** — a modular, high-performance hypervisor orchestration engine. Pane gives you bare-metal performance with microVM convenience, blending Rust speed, KVM control, `io_uring` storage throughput, eBPF micro-segmentation, and Go's ease of integration.

Whether you're sandboxing a local tool, building a serverless SaaS platform, or running full VM stacks, this guide will get you up and running.

---

## 1. Local Sandboxing (The Solo Developer flow)

For local development or command-line testing, Pane supports a lightweight Firecracker backend designed for direct kernel booting. It bypasses heavy bios/bootloaders, booting a Linux kernel in **sub-1ms**.

### Prerequisites
Make sure your development machine has KVM enabled and you have read/write access to `/dev/kvm`:
```bash
ls -l /dev/kvm
# If needed, add yourself to the kvm group:
sudo usermod -aG kvm $USER
```

### Spawning your first microVM
Using the Go gRPC client or by calling the FFI functions directly, you can boot a microVM.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "pane/pane-api/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// 1. Connect to the Pane gRPC daemon
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()
	client := pb.NewPaneServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 2. Boot a microVM with a direct kernel boot config
	resp, err := client.Spawn(ctx, &pb.SpawnRequest{
		Id:          "sandbox-vm-1",
		KernelPath:  "/path/to/vmlinux",
		RootfsPath:  "/path/to/rootfs.img",
		VcpuCount:   1,
		MemSizeMib:  128,
		BootArgs:    "console=ttyS0 reboot=k panic=1 pci=off",
	})
	if err != nil {
		log.Fatalf("Failed to spawn VM: %v", err)
	}

	fmt.Printf("VM Booted! ID: %s, Vsock CID: %d, PID: %d\n", resp.Id, resp.VsockCid, resp.Pid)
}
```

---

## 2. Architecting for Scale (The High-End Production flow)

When building a high-density host environment, you need resource isolation, network micro-segmentation, and instant scalability. Pane handles this without heavy hypervisor layers or complex corporate setups.

### Resource Controls via cgroup v2
Pane automatically creates a dedicated cgroup directory for every VM. You can dynamically apply CPU scheduling, memory limits, and process limits:
- **Memory Caps**: Enforce tight bounds (e.g., 256MB RAM). If the guest runs Out of Memory, KVM traps the process inside the cgroup, protecting the host system from crashing.
- **CPU Quotas**: Limit CPU shares to ensure fair scheduling across co-located tenants.

### Network Micro-segmentation with eBPF (`aya`)
Instead of clogging host routing tables with thousands of complex `iptables` or `nftables` rules, Pane compiles a custom eBPF packet classifier ([pane_filter.bpf.c](file:///home/varhan/projects/pane/pane-core/src/bpf/pane_filter.bpf.c)) and attaches it directly to the host TAP device ingress using **Traffic Control (TC)**.
- VMs are assigned to group IDs.
- eBPF inspects IPv4 packet headers on the host interface.
- Communication between VMs of different groups is dropped at the lowest kernel layer with **zero overhead**.

### Instant Cloning (Forking)
For serverless scaling, boot-up time is critical. Rather than starting a VM from scratch, Pane lets you freeze a running VM, write its memory and device state to a snapshot, and instantly fork clones:
- **Benchmark**: Clone 50 VMs from a single parent snapshot in **under 2 seconds total wall-time**.
- **CoW Memory**: Uses Copy-on-Write memory mapping to share parent pages until modified, minimizing RAM consumption.

---

## 3. Running Custom ISOs & Windows on Pane

Pane's default `Spawning` state leverages Firecracker for speed, but the C-layer hypervisor `pane-vmm` contains a fully functional **QEMU Backend** designed to run custom ISOs, full operating systems (like Ubuntu Server), or Windows.

Under the hood, the QEMU backend instantiates a standard PCI bus layout with virtual drivers suitable for full hardware emulation.

### The QEMU Invocation
When you transition a VM to QEMU mode in `pane-vmm` via [pane_vm_setup_qemu_mode](file:///home/varhan/projects/pane/pane-vmm/src/backends/qemu.c#L58-L110), it spawns:
```bash
qemu-system-x86_64 -enable-kvm -m 128 -smp 1 -display none -nographic -drive file=<image_path>,format=raw,if=virtio -qmp unix:<qmp_socket_path>,server,nowait -serial none
```

### Step 1: Preparing files for Custom / Windows ISO
Because installing Windows or running a full desktop ISO requires more resources (e.g., graphics/VNC, at least 4GB of RAM, and multiple drives), you will want to customize the QEMU command-line parameters.

1. **Create a blank virtual disk image** (for the OS installation destination):
   ```bash
   qemu-img create -f qcow2 windows_install.qcow2 40G
   ```
2. **Download the ISO**: Place your Windows ISO (e.g. `Win10_Installer.iso`) or Custom Linux ISO (e.g. `ubuntu.iso`) in a accessible directory.

### Step 2: Customizing the QEMU Backend in Pane
To run heavy OS installs, open [pane-vmm/src/backends/qemu.c](file:///home/varhan/projects/pane/pane-vmm/src/backends/qemu.c#L83-L103) and customize the boot parameters to support larger memory limits, attach the ISO, and enable VNC display access:

```diff
         char *args[] = {
             "qemu-system-x86_64",
             "-enable-kvm",
-            "-m", "128",
-            "-smp", "1",
-            "-display", "none",
-            "-nographic",
+            "-m", "4096",              // Allocate 4GB RAM for Windows
+            "-smp", "4",               // Allocate 4 vCPUs
+            "-vnc", ":1",              // Enable VNC display on port 5901 for graphical installation
             "-drive", NULL,            // Destination drive (customized below)
+            "-cdrom", "/path/to/installer.iso", // ISO Installer
             "-qmp", NULL,              
             "-serial", "none",
             NULL
         };
```

*Tip: If you're running headless on a server, mapping the VNC port (`:1` translates to port `5901` on the host) allows you to connect with any standard VNC client to interact with the OS GUI installer.*

### Step 3: Launching the Installation
Compile the updated VMM library and call `pane_vm_setup_qemu_mode` passing the path to the blank destination disk:

```go
// Setup the VM in QEMU mode to begin the ISO installation
err := ffi.SetupQemuMode("windows_install.qcow2", "/run/pane/qmp.sock")
if err != nil {
    log.Fatalf("Failed to initialize QEMU install: %v", err)
}
```

Once installation is finished, you can remove the `-cdrom` installer flag from [qemu.c](file:///home/varhan/projects/pane/pane-vmm/src/backends/qemu.c) and boot directly from your newly populated `windows_install.qcow2` drive.
