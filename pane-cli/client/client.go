// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"encoding/json"
	"io"
	"pane/pane-api/ffi"
	"pane/pane-api/panespec"
	pb "pane/pane-api/proto"

	"google.golang.org/grpc"
)

// DaemonClient defines the common interface for client-side VM operations.
type DaemonClient interface {
	Spawn(ctx context.Context, id string, spec *panespec.PaneSpec) (uint32, uint32, error)
	Exec(ctx context.Context, id, command string, args []string, cb func(data []byte, isStderr bool, exitCode int32)) error
	Snapshot(ctx context.Context, id, snapshotPath, memFilePath string) error
	Fork(ctx context.Context, id, snapshotPath, memFilePath, newRootfsPath string, newVsockCid uint32) (uint32, error)
	Destroy(ctx context.Context, id string) error
}

// EmbeddedClient runs operations in-process via CGo FFI bindings directly into pane-core.
type EmbeddedClient struct{}

func NewEmbeddedClient() *EmbeddedClient {
	return &EmbeddedClient{}
}

func (c *EmbeddedClient) Spawn(ctx context.Context, id string, spec *panespec.PaneSpec) (uint32, uint32, error) {
	return ffi.Spawn(id, spec)
}

func (c *EmbeddedClient) Exec(ctx context.Context, id, command string, args []string, cb func(data []byte, isStderr bool, exitCode int32)) error {
	return ffi.Exec(id, command, args, ffi.ExecCallback(cb))
}

func (c *EmbeddedClient) Snapshot(ctx context.Context, id, snapshotPath, memFilePath string) error {
	return ffi.Snapshot(id, snapshotPath, memFilePath)
}

func (c *EmbeddedClient) Fork(ctx context.Context, id, snapshotPath, memFilePath, newRootfsPath string, newVsockCid uint32) (uint32, error) {
	return ffi.Fork(id, snapshotPath, memFilePath, newRootfsPath, newVsockCid)
}

func (c *EmbeddedClient) Destroy(ctx context.Context, id string) error {
	return ffi.Destroy(id)
}

// GrpcClient runs operations by calling the pane daemon via gRPC.
type GrpcClient struct {
	client pb.PaneServiceClient
}

func NewGrpcClient(conn *grpc.ClientConn) *GrpcClient {
	return &GrpcClient{
		client: pb.NewPaneServiceClient(conn),
	}
}

func (c *GrpcClient) Spawn(ctx context.Context, id string, spec *panespec.PaneSpec) (uint32, uint32, error) {
	kernel := ""
	if spec.Kernel != nil {
		kernel = *spec.Kernel
	}
	rootfs := ""
	if spec.Disk != nil && spec.Disk.Path != nil {
		rootfs = *spec.Disk.Path
	}
	bootArgs := ""
	if spec.Cmdline != nil {
		bootArgs = *spec.Cmdline
	}
	vcpu := uint32(1)
	if spec.CPUs != nil {
		vcpu = *spec.CPUs
	}
	mem := uint32(128)
	if spec.Memory != nil {
		if val, err := panespec.ParseSize(*spec.Memory); err == nil {
			mem = uint32(val / (1024 * 1024))
		}
	}

	specJson := ""
	if specBytes, err := json.Marshal(spec); err == nil {
		specJson = string(specBytes)
	}

	resp, err := c.client.Spawn(ctx, &pb.SpawnRequest{
		Id:         id,
		KernelPath: kernel,
		RootfsPath: rootfs,
		VcpuCount:  vcpu,
		MemSizeMib: mem,
		BootArgs:   bootArgs,
		SpecJson:   specJson,
	})
	if err != nil {
		return 0, 0, err
	}
	return resp.VsockCid, resp.Pid, nil
}

func (c *GrpcClient) Exec(ctx context.Context, id, command string, args []string, cb func(data []byte, isStderr bool, exitCode int32)) error {
	stream, err := c.client.Exec(ctx, &pb.ExecRequest{
		Id:      id,
		Command: command,
		Args:    args,
	})
	if err != nil {
		return err
	}
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		cb(resp.Data, resp.IsStderr, resp.ExitCode)
	}
	return nil
}

func (c *GrpcClient) Snapshot(ctx context.Context, id, snapshotPath, memFilePath string) error {
	_, err := c.client.Snapshot(ctx, &pb.SnapshotRequest{
		Id:           id,
		SnapshotPath: snapshotPath,
		MemFilePath:  memFilePath,
	})
	return err
}

func (c *GrpcClient) Fork(ctx context.Context, id, snapshotPath, memFilePath, newRootfsPath string, newVsockCid uint32) (uint32, error) {
	resp, err := c.client.Fork(ctx, &pb.ForkRequest{
		Id:            id,
		SnapshotPath:  snapshotPath,
		MemFilePath:   memFilePath,
		NewRootfsPath: newRootfsPath,
		NewVsockCid:   newVsockCid,
	})
	if err != nil {
		return 0, err
	}
	return resp.Pid, nil
}

func (c *GrpcClient) Destroy(ctx context.Context, id string) error {
	_, err := c.client.Destroy(ctx, &pb.DestroyRequest{
		Id: id,
	})
	return err
}
