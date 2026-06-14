use crate::backends::{BootSource, Drive, MachineConfig};
use crate::error::PaneError;
use crate::exec::ExecChunk;
use crate::vm::{Running, Spawning, Vm};
use crate::ffi::vmm::pane_vmm_config_t;
use crate::ffi::SafeVm;
use std::ffi::CStr;
use tokio::runtime::Runtime;

fn error_to_errno(err: PaneError) -> libc::c_int {
    match err {
        PaneError::Vmm(code, _) => -code,
        PaneError::Io(e) => -e.raw_os_error().unwrap_or(libc::EIO),
        PaneError::Json(_) => -libc::EINVAL,
        PaneError::Socket(_) => -libc::ENOTCONN,
        PaneError::Spawn(_) => -libc::EAGAIN,
        PaneError::Api { .. } => -libc::EBADMSG,
        PaneError::Timeout(_) => -libc::ETIMEDOUT,
    }
}

/// Spawns a new Firecracker-backed VM from parameters.
/// Returns 0 on success, negative errno on failure.
///
/// # Safety
/// This function dereferences raw pointers. The caller must ensure that `id`,
/// `kernel_path`, `rootfs_path`, and `boot_args` (if not null) are valid,
/// null-terminated C strings. `cid_out` and `pid_out` must point to valid,
/// writable memory locations.
#[no_mangle]
pub unsafe extern "C" fn pane_core_spawn(
    id: *const libc::c_char,
    config: *const pane_vmm_config_t,
    cid_out: *mut u32,
    pid_out: *mut u32,
) -> libc::c_int {
    if id.is_null() || config.is_null() {
        return -libc::EINVAL;
    }

    let rt = match Runtime::new() {
        Ok(r) => r,
        Err(_) => return -libc::ENOMEM,
    };

    let cfg = &*config;

    rt.block_on(async {
        let id_str = match CStr::from_ptr(id).to_str() {
            Ok(s) => s,
            Err(_) => return -libc::EINVAL,
        };

        let vmm_type_str = if cfg.vmm_type.is_null() {
            "firecracker"
        } else {
            match CStr::from_ptr(cfg.vmm_type).to_str() {
                Ok(s) => s,
                Err(_) => "firecracker",
            }
        };

        if vmm_type_str == "qemu" {
            // Spawn QEMU backend
            let safe_vm = match SafeVm::create() {
                Ok(vm) => vm,
                Err(e) => return error_to_errno(e),
            };

            let qmp_socket_path = format!("/run/pane/qmp-{}.sock", id_str);
            let qmp_path = std::path::Path::new(&qmp_socket_path);
            if let Some(parent) = qmp_path.parent() {
                let _ = std::fs::create_dir_all(parent);
            }

            if let Err(e) = safe_vm.setup_qemu_mode(config, &qmp_socket_path) {
                return error_to_errno(e);
            }

            let vm = Vm::new_native(id_str, safe_vm);
            let running_vm = match vm.start().await {
                Ok(r) => r,
                Err(e) => return error_to_errno(e),
            };

            if !cid_out.is_null() {
                *cid_out = running_vm.vsock_cid();
            }
            if !pid_out.is_null() {
                *pid_out = running_vm.pid().unwrap_or(0);
            }

            std::mem::forget(running_vm);
            0
        } else {
            // Spawn Firecracker backend
            let kernel_str = if cfg.kernel_path.is_null() {
                ""
            } else {
                match CStr::from_ptr(cfg.kernel_path).to_str() {
                    Ok(s) => s,
                    Err(_) => return -libc::EINVAL,
                }
            };
            let disk_str = if cfg.disk_path.is_null() {
                ""
            } else {
                match CStr::from_ptr(cfg.disk_path).to_str() {
                    Ok(s) => s,
                    Err(_) => return -libc::EINVAL,
                }
            };
            let boot_args_str = if cfg.cmdline.is_null() {
                None
            } else {
                match CStr::from_ptr(cfg.cmdline).to_str() {
                    Ok(s) => Some(s.to_string()),
                    Err(_) => return -libc::EINVAL,
                }
            };

            let mut vm = Vm::new_firecracker(id_str);
            if let Err(e) = vm.spawn().await {
                return error_to_errno(e);
            }

            let machine_config = MachineConfig {
                vcpu_count: cfg.vcpus,
                mem_size_mib: (cfg.memory_bytes / (1024 * 1024)) as u32,
                smt: Some(false),
                track_dirty_pages: Some(true),
            };
            if let Err(e) = vm.configure_machine(&machine_config).await {
                return error_to_errno(e);
            }

            if !kernel_str.is_empty() {
                let boot_source = BootSource {
                    kernel_image_path: kernel_str.to_string(),
                    boot_args: boot_args_str,
                };
                if let Err(e) = vm.configure_boot_source(&boot_source).await {
                    return error_to_errno(e);
                }
            }

            if !disk_str.is_empty() {
                let drive = Drive {
                    drive_id: "rootfs".to_string(),
                    path_on_host: disk_str.to_string(),
                    is_root_device: true,
                    is_read_only: false,
                };
                if let Err(e) = vm.configure_drive(&drive).await {
                    return error_to_errno(e);
                }
            }

            // Configure a default guest CID (3)
            let guest_cid = 3;
            if let Err(e) = vm.configure_vsock(guest_cid).await {
                return error_to_errno(e);
            }

            let running_vm = match vm.start().await {
                Ok(r) => r,
                Err(e) => return error_to_errno(e),
            };

            if !cid_out.is_null() {
                *cid_out = running_vm.vsock_cid();
            }
            if !pid_out.is_null() {
                *pid_out = running_vm.pid().unwrap_or(0);
            }

            std::mem::forget(running_vm);
            0
        }
    })
}

/// Takes a snapshot of a running VM.
/// Returns 0 on success, negative errno on failure.
///
/// # Safety
/// This function dereferences raw pointers. The caller must ensure that `id`,
/// `snapshot_path`, and `mem_path` point to valid, null-terminated C strings.
#[no_mangle]
pub unsafe extern "C" fn pane_core_snapshot(
    id: *const libc::c_char,
    snapshot_path: *const libc::c_char,
    mem_path: *const libc::c_char,
) -> libc::c_int {
    if id.is_null() || snapshot_path.is_null() || mem_path.is_null() {
        return -libc::EINVAL;
    }

    let rt = match Runtime::new() {
        Ok(r) => r,
        Err(_) => return -libc::ENOMEM,
    };

    rt.block_on(async {
        let id_str = match CStr::from_ptr(id).to_str() {
            Ok(s) => s,
            Err(_) => return -libc::EINVAL,
        };
        let snap_str = match CStr::from_ptr(snapshot_path).to_str() {
            Ok(s) => s,
            Err(_) => return -libc::EINVAL,
        };
        let mem_str = match CStr::from_ptr(mem_path).to_str() {
            Ok(s) => s,
            Err(_) => return -libc::EINVAL,
        };

        let running_vm = Vm::<Running>::assume_running(id_str);

        let frozen_vm = match running_vm.freeze().await {
            Ok(f) => f,
            Err(e) => return error_to_errno(e),
        };

        if let Err(e) = frozen_vm.create_snapshot(snap_str, mem_str).await {
            let _ = frozen_vm.resume().await; // Try to resume anyway
            return error_to_errno(e);
        }

        if let Err(e) = frozen_vm.resume().await {
            return error_to_errno(e);
        }

        0
    })
}

/// Clones/forks a VM from a snapshot.
/// Returns 0 on success, negative errno on failure.
///
/// # Safety
/// This function dereferences raw pointers. The caller must ensure that `id`,
/// `snapshot_path`, `mem_path`, and `new_rootfs` point to valid, null-terminated
/// C strings, and that `pid_out` points to a valid, writable memory location.
#[no_mangle]
pub unsafe extern "C" fn pane_core_fork(
    id: *const libc::c_char,
    snapshot_path: *const libc::c_char,
    mem_path: *const libc::c_char,
    new_rootfs: *const libc::c_char,
    new_cid: u32,
    pid_out: *mut u32,
) -> libc::c_int {
    if id.is_null() || snapshot_path.is_null() || mem_path.is_null() || new_rootfs.is_null() {
        return -libc::EINVAL;
    }

    let rt = match Runtime::new() {
        Ok(r) => r,
        Err(_) => return -libc::ENOMEM,
    };

    rt.block_on(async {
        let id_str = match CStr::from_ptr(id).to_str() {
            Ok(s) => s,
            Err(_) => return -libc::EINVAL,
        };
        let snap_str = match CStr::from_ptr(snapshot_path).to_str() {
            Ok(s) => s,
            Err(_) => return -libc::EINVAL,
        };
        let mem_str = match CStr::from_ptr(mem_path).to_str() {
            Ok(s) => s,
            Err(_) => return -libc::EINVAL,
        };
        let rootfs_str = match CStr::from_ptr(new_rootfs).to_str() {
            Ok(s) => s,
            Err(_) => return -libc::EINVAL,
        };

        let mut frozen_vm = match Vm::<Spawning>::fork_firecracker(id_str, snap_str, mem_str).await
        {
            Ok(f) => f,
            Err(e) => return error_to_errno(e),
        };

        if let Err(e) = frozen_vm.patch_drive("rootfs", rootfs_str).await {
            let _ = frozen_vm.destroy().await;
            return error_to_errno(e);
        }

        if let Err(e) = frozen_vm.configure_vsock(new_cid).await {
            let _ = frozen_vm.destroy().await;
            return error_to_errno(e);
        }

        let running_vm = match frozen_vm.resume().await {
            Ok(r) => r,
            Err(e) => return error_to_errno(e),
        };

        if !pid_out.is_null() {
            *pid_out = running_vm.pid().unwrap_or(0);
        }

        std::mem::forget(running_vm);

        0
    })
}

/// Terminates and cleans up resources for a VM.
/// Returns 0 on success, negative errno on failure.
///
/// # Safety
/// This function dereferences raw pointers. The caller must ensure that `id`
/// points to a valid, null-terminated C string.
#[no_mangle]
pub unsafe extern "C" fn pane_core_destroy(id: *const libc::c_char) -> libc::c_int {
    if id.is_null() {
        return -libc::EINVAL;
    }

    let rt = match Runtime::new() {
        Ok(r) => r,
        Err(_) => return -libc::ENOMEM,
    };

    rt.block_on(async {
        let id_str = match CStr::from_ptr(id).to_str() {
            Ok(s) => s,
            Err(_) => return -libc::EINVAL,
        };

        let running_vm = Vm::<Running>::assume_running(id_str);
        if let Err(e) = running_vm.destroy().await {
            return error_to_errno(e);
        }

        0
    })
}

/// Runs a command inside a running VM via vsock, invoking the callback for output chunks.
/// Returns 0 on success, negative errno on failure.
///
/// # Safety
/// This function dereferences raw pointers and invokes a C callback. The caller
/// must ensure that `id`, `command`, and `args_json` point to valid, null-terminated
/// C strings. `callback` must be a valid, callable function pointer.
#[no_mangle]
pub unsafe extern "C" fn pane_core_exec(
    id: *const libc::c_char,
    command: *const libc::c_char,
    args_json: *const libc::c_char,
    callback: extern "C" fn(
        data: *const u8,
        len: usize,
        is_stderr: i32,
        exit_code: i32,
        user_data: *mut libc::c_void,
    ) -> i32,
    user_data: *mut libc::c_void,
) -> libc::c_int {
    if id.is_null() || command.is_null() || args_json.is_null() {
        return -libc::EINVAL;
    }

    let rt = match Runtime::new() {
        Ok(r) => r,
        Err(_) => return -libc::ENOMEM,
    };

    rt.block_on(async {
        let id_str = match CStr::from_ptr(id).to_str() {
            Ok(s) => s,
            Err(_) => return -libc::EINVAL,
        };
        let cmd_str = match CStr::from_ptr(command).to_str() {
            Ok(s) => s,
            Err(_) => return -libc::EINVAL,
        };
        let args_str = match CStr::from_ptr(args_json).to_str() {
            Ok(s) => s,
            Err(_) => return -libc::EINVAL,
        };

        let args: Vec<String> = match serde_json::from_str(args_str) {
            Ok(a) => a,
            Err(_) => return -libc::EINVAL,
        };

        let running_vm = Vm::<Running>::assume_running(id_str);
        let req = crate::exec::ExecRequest {
            command: cmd_str.to_string(),
            args,
        };

        let mut stream = match running_vm.exec(&req).await {
            Ok(s) => s,
            Err(e) => return error_to_errno(e),
        };

        loop {
            match stream.next().await {
                Ok(Some(chunk)) => {
                    let (data_ptr, data_len, is_stderr, exit_code) = match &chunk {
                        ExecChunk::Stdout(data) => (data.as_ptr(), data.len(), 0, -1),
                        ExecChunk::Stderr(data) => (data.as_ptr(), data.len(), 1, -1),
                        ExecChunk::ExitCode(code) => (std::ptr::null(), 0, 0, *code),
                    };

                    let ret = callback(data_ptr, data_len, is_stderr, exit_code, user_data);
                    if ret != 0 {
                        // Callback cancelled or failed
                        return -libc::ECANCELED;
                    }
                    if let ExecChunk::ExitCode(_) = chunk {
                        break;
                    }
                }
                Ok(None) => break,
                Err(e) => return error_to_errno(e),
            }
        }

        0
    })
}
