package ffi

/*
#cgo LDFLAGS: -L../../pane-core/target/debug -lpane_core -L../../pane-vmm -lpane_vmm -luring -ldl -lpthread -lm
#include <stdint.h>
#include <stdlib.h>

typedef int (*extern_callback_t)(const uint8_t* data, size_t len, int is_stderr, int exit_code, void* user_data);

int pane_core_spawn(const char* id, const char* kernel, const char* rootfs, uint32_t vcpu, uint32_t mem, const char* boot_args, uint32_t* cid_out, uint32_t* pid_out);
int pane_core_snapshot(const char* id, const char* snap_path, const char* mem_path);
int pane_core_fork(const char* id, const char* snapshot_path, const char* mem_path, const char* new_rootfs, uint32_t new_cid, uint32_t* pid_out);
int pane_core_destroy(const char* id);
int pane_core_exec(const char* id, const char* command, const char* args_json, extern_callback_t callback, void* user_data);

// Gateway function declaration for CGo callback
int gatewayExecCallback(uint8_t* data, size_t len, int isStderr, int exitCode, void* userData);
*/
import "C"
import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime/cgo"
	"syscall"
	"unsafe"
)

// Helper to convert negative errno return value to a Go error
func checkErr(code C.int, context string) error {
	if code < 0 {
		errno := syscall.Errno(-code)
		return fmt.Errorf("%s failed: %w", context, errno)
	}
	return nil
}

// Spawn boots a VM via Rust orchestration FFI
func Spawn(id, kernel, rootfs, bootArgs string, vcpu, mem uint32) (uint32, uint32, error) {
	cId := C.CString(id)
	defer C.free(unsafe.Pointer(cId))
	cKernel := C.CString(kernel)
	defer C.free(unsafe.Pointer(cKernel))
	cRootfs := C.CString(rootfs)
	defer C.free(unsafe.Pointer(cRootfs))

	var cBootArgs *C.char
	if bootArgs != "" {
		cBootArgs = C.CString(bootArgs)
		defer C.free(unsafe.Pointer(cBootArgs))
	}

	var cid C.uint32_t
	var pid C.uint32_t

	ret := C.pane_core_spawn(cId, cKernel, cRootfs, C.uint32_t(vcpu), C.uint32_t(mem), cBootArgs, &cid, &pid)
	if err := checkErr(ret, "Spawn"); err != nil {
		return 0, 0, err
	}

	return uint32(cid), uint32(pid), nil
}

// Snapshot pauses VM and takes a state snapshot
func Snapshot(id, snapshotPath, memPath string) error {
	cId := C.CString(id)
	defer C.free(unsafe.Pointer(cId))
	cSnap := C.CString(snapshotPath)
	defer C.free(unsafe.Pointer(cSnap))
	cMem := C.CString(memPath)
	defer C.free(unsafe.Pointer(cMem))

	ret := C.pane_core_snapshot(cId, cSnap, cMem)
	return checkErr(ret, "Snapshot")
}

// Fork clones VM from a snapshot
func Fork(id, snapshotPath, memPath, newRootfs string, newCid uint32) (uint32, error) {
	cId := C.CString(id)
	defer C.free(unsafe.Pointer(cId))
	cSnap := C.CString(snapshotPath)
	defer C.free(unsafe.Pointer(cSnap))
	cMem := C.CString(memPath)
	defer C.free(unsafe.Pointer(cMem))
	cRootfs := C.CString(newRootfs)
	defer C.free(unsafe.Pointer(cRootfs))

	var pid C.uint32_t

	ret := C.pane_core_fork(cId, cSnap, cMem, cRootfs, C.uint32_t(newCid), &pid)
	if err := checkErr(ret, "Fork"); err != nil {
		return 0, err
	}

	return uint32(pid), nil
}

// Destroy kills a VM and frees resources
func Destroy(id string) error {
	if id == "" {
		return errors.New("VM ID cannot be empty")
	}
	cId := C.CString(id)
	defer C.free(unsafe.Pointer(cId))

	ret := C.pane_core_destroy(cId)
	return checkErr(ret, "Destroy")
}

// ExecCallback defines a function signature to handle streamed execution chunks in Go
type ExecCallback func(data []byte, isStderr bool, exitCode int32)

// Exec runs a command inside the VM and streams stdout/stderr back in real-time
func Exec(id, command string, args []string, cb ExecCallback) error {
	cId := C.CString(id)
	defer C.free(unsafe.Pointer(cId))
	cCmd := C.CString(command)
	defer C.free(unsafe.Pointer(cCmd))

	// Serialize args to JSON array
	argsJson := "[]"
	if len(args) > 0 {
		bytes, err := json.Marshal(args)
		if err != nil {
			return err
		}
		argsJson = string(bytes)
	}
	cArgs := C.CString(argsJson)
	defer C.free(unsafe.Pointer(cArgs))

	// Wrap Go callback closure in cgo.Handle
	handle := cgo.NewHandle(cb)
	defer handle.Delete()

	ret := C.pane_core_exec(cId, cCmd, cArgs, (C.extern_callback_t)(unsafe.Pointer(C.gatewayExecCallback)), unsafe.Pointer(&handle))
	return checkErr(ret, "Exec")
}

//export gatewayExecCallback
func gatewayExecCallback(data *C.uint8_t, len C.size_t, isStderr C.int, exitCode C.int, userData unsafe.Pointer) C.int {
	hPtr := (*cgo.Handle)(userData)
	cb := hPtr.Value().(ExecCallback)

	var goBytes []byte
	if data != nil && len > 0 {
		goBytes = C.GoBytes(unsafe.Pointer(data), C.int(len))
	}

	cb(goBytes, isStderr != 0, int32(exitCode))
	return 0
}
