// SPDX-License-Identifier: Apache-2.0

pub mod backends;
pub mod error;
pub mod exec;
pub mod ffi;
pub mod network;
pub mod resources;
pub mod vm;

pub use backends::{
    BootSource, Drive, FirecrackerVm, MachineConfig, NetworkInterfaceConfig, VsockConfig,
};
pub use error::{check_ffi, PaneError, Result};
pub use exec::{exec_in_guest, ExecChunk, ExecRequest, ExecStream};
pub use ffi::{pane_backend_t, SafeVm};
pub use network::{
    attach_filter_to_interface, get_vm_network_group, init_network_ebpf, register_vm_network_group,
    unregister_vm_network_group,
};
pub use resources::{CgroupManager, CpuMaxLimit, ResourceControls};
pub use vm::{cow_clone_rootfs, Dead, Frozen, Running, Spawning, Vm, VmBackend, VmState};

pub(crate) fn is_run_pane_writable() -> bool {
    let test_file = std::path::Path::new("/run/pane/.tmp_write_test");
    if std::fs::write(test_file, "").is_ok() {
        let _ = std::fs::remove_file(test_file);
        true
    } else {
        false
    }
}
