// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"

	"pane/pane-api/ffi"
	"pane/pane-api/panespec"
	pb "pane/pane-api/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var envMutex sync.Mutex

// PaneServer implements the gRPC PaneService interface
type PaneServer struct {
	pb.UnimplementedPaneServiceServer
}

// Spawn handles booting a new microVM instance
func (s *PaneServer) Spawn(ctx context.Context, req *pb.SpawnRequest) (*pb.SpawnResponse, error) {
	var spec *panespec.PaneSpec
	if req.SpecJson != "" {
		spec = &panespec.PaneSpec{}
		if err := json.Unmarshal([]byte(req.SpecJson), spec); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "failed to parse spec_json: %v", err)
		}
	} else {
		if req.Id == "" || req.KernelPath == "" || req.RootfsPath == "" {
			return nil, status.Error(codes.InvalidArgument, "id, kernel_path, and rootfs_path cannot be empty")
		}
		spec = &panespec.PaneSpec{
			VMM:    panespec.PtrVMMType(panespec.VMMFirecracker),
			CPUs:   panespec.PtrUint32(req.VcpuCount),
			Memory: panespec.PtrString(fmt.Sprintf("%dMiB", req.MemSizeMib)),
			Disk: &panespec.DiskConfig{
				Path:   panespec.PtrString(req.RootfsPath),
				Format: panespec.PtrDiskFormat(panespec.FormatRaw),
			},
			Kernel:  panespec.PtrString(req.KernelPath),
			Cmdline: panespec.PtrString(req.BootArgs),
		}
	}

	envMutex.Lock()
	originalEnv := make(map[string]*string)
	if spec.Env != nil {
		for k, v := range spec.Env {
			// Fixed environment leak where we indiscriminately unset variables that might have already existed
			if origV, exists := os.LookupEnv(k); exists {
				originalEnv[k] = &origV
			} else {
				originalEnv[k] = nil
			}
			os.Setenv(k, v)
		}
	}

	cid, pid, err := ffi.Spawn(req.Id, spec)

	if spec.Env != nil {
		for k := range spec.Env {
			if origV, ok := originalEnv[k]; ok && origV != nil {
				os.Setenv(k, *origV)
			} else {
				os.Unsetenv(k)
			}
		}
	}
	envMutex.Unlock()
	if err != nil {
		// Attempt to read QEMU logs to give a more informative error message
		logPath := fmt.Sprintf("/run/pane/qemu-%s.log", req.Id)
		if _, errStat := os.Stat(logPath); os.IsNotExist(errStat) {
			logPath = fmt.Sprintf("/tmp/pane/qemu-%s.log", req.Id)
		}
		if logBytes, errRead := os.ReadFile(logPath); errRead == nil && len(logBytes) > 0 {
			_ = os.Remove(logPath)
			return nil, status.Errorf(codes.Internal, "failed to spawn VM: %v\nQEMU log:\n%s", err, string(logBytes))
		}
		return nil, status.Errorf(codes.Internal, "failed to spawn VM: %v", err)
	}

	return &pb.SpawnResponse{
		Id:       req.Id,
		VsockCid: cid,
		Pid:      pid,
	}, nil
}

// Exec executes a command inside the VM and streams stdout/stderr chunks back
func (s *PaneServer) Exec(req *pb.ExecRequest, stream pb.PaneService_ExecServer) error {
	if req.Id == "" || req.Command == "" {
		return status.Error(codes.InvalidArgument, "id and command cannot be empty")
	}

	// Channel to receive gRPC chunks from the CGo callback
	type grpcChunk struct {
		data     []byte
		isStderr bool
		exitCode int32
		err      error
	}
	ch := make(chan grpcChunk, 16)

	// Define Go callback
	callback := func(data []byte, isStderr bool, exitCode int32) {
		ch <- grpcChunk{
			data:     data,
			isStderr: isStderr,
			exitCode: exitCode,
		}
	}

	// Run CGo execution in a background goroutine so we can select on context cancellation
	go func() {
		err := ffi.Exec(req.Id, req.Command, req.Args, callback)
		if err != nil {
			ch <- grpcChunk{err: err}
		}
		close(ch)
	}()

	// Loop and stream chunks to gRPC client
	for {
		select {
		case <-stream.Context().Done():
			return status.Error(codes.Canceled, "stream execution canceled by client")
		case chunk, ok := <-ch:
			if !ok {
				return nil
			}
			if chunk.err != nil {
				return status.Errorf(codes.Internal, "execution error: %v", chunk.err)
			}

			resp := &pb.ExecResponse{
				Data:     chunk.data,
				IsStderr: chunk.isStderr,
				ExitCode: chunk.exitCode,
			}

			if err := stream.Send(resp); err != nil {
				return status.Errorf(codes.Unavailable, "failed to send gRPC stream message: %v", err)
			}

			if chunk.exitCode >= 0 {
				return nil // Finished cleanly
			}
		}
	}
}

// Snapshot freezes execution and captures state to files
func (s *PaneServer) Snapshot(ctx context.Context, req *pb.SnapshotRequest) (*pb.SnapshotResponse, error) {
	if req.Id == "" || req.SnapshotPath == "" || req.MemFilePath == "" {
		return nil, status.Error(codes.InvalidArgument, "id, snapshot_path, and mem_file_path cannot be empty")
	}

	err := ffi.Snapshot(req.Id, req.SnapshotPath, req.MemFilePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to take snapshot: %v", err)
	}

	return &pb.SnapshotResponse{
		Id: req.Id,
	}, nil
}

// Fork creates a microVM clone from a snapshot
func (s *PaneServer) Fork(ctx context.Context, req *pb.ForkRequest) (*pb.ForkResponse, error) {
	if req.Id == "" || req.SnapshotPath == "" || req.MemFilePath == "" || req.NewRootfsPath == "" {
		return nil, status.Error(codes.InvalidArgument, "missing required fields for fork request")
	}

	pid, err := ffi.Fork(req.Id, req.SnapshotPath, req.MemFilePath, req.NewRootfsPath, req.NewVsockCid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fork VM: %v", err)
	}

	return &pb.ForkResponse{
		Id:       req.Id,
		VsockCid: req.NewVsockCid,
		Pid:      pid,
	}, nil
}

// Destroy stops microVM process and cleans up cgroups/sockets
func (s *PaneServer) Destroy(ctx context.Context, req *pb.DestroyRequest) (*pb.DestroyResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id cannot be empty")
	}

	err := ffi.Destroy(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to destroy VM: %v", err)
	}

	return &pb.DestroyResponse{
		Id: req.Id,
	}, nil
}

// StartGrpcServer runs the gRPC listener on the specified port
func StartGrpcServer(port int) (*grpc.Server, error) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %d: %w", port, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterPaneServiceServer(grpcServer, &PaneServer{})

	go func() {
		if err := grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			fmt.Printf("gRPC server error: %v\n", err)
		}
	}()

	return grpcServer, nil
}

// StartGrpcServerUnix runs the gRPC listener on the specified UNIX domain socket path
func StartGrpcServerUnix(socketPath string) (*grpc.Server, error) {
	// Clean up stale socket file if it exists
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to remove stale socket: %w", err)
	}

	lis, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on unix socket %s: %w", socketPath, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterPaneServiceServer(grpcServer, &PaneServer{})

	go func() {
		if err := grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			fmt.Printf("gRPC server error: %v\n", err)
		}
	}()

	return grpcServer, nil
}
