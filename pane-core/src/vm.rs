use crate::backends::FirecrackerVm;
use crate::error::Result;
use crate::ffi::SafeVm;
use std::marker::PhantomData;

/// Opaque marker trait representing a valid state for a Virtual Machine.
pub trait VmState: private::Sealed {}

mod private {
    pub trait Sealed {}
}

/// Represents a Virtual Machine in the Spawning state (being configured).
///
/// # Example
/// ```no_run
/// use pane_core::vm::Spawning;
/// ```
#[derive(Debug)]
pub struct Spawning;
impl VmState for Spawning {}
impl private::Sealed for Spawning {}

/// Represents a Virtual Machine in the Running state (active execution).
///
/// # Example
/// ```no_run
/// use pane_core::vm::Running;
/// ```
#[derive(Debug)]
pub struct Running;
impl VmState for Running {}
impl private::Sealed for Running {}

/// Represents a Virtual Machine in the Frozen state (paused execution).
///
/// # Example
/// ```no_run
/// use pane_core::vm::Frozen;
/// ```
#[derive(Debug)]
pub struct Frozen;
impl VmState for Frozen {}
impl private::Sealed for Frozen {}

/// Represents a Virtual Machine in the Dead state (terminated, resources reclaimed).
///
/// # Example
/// ```no_run
/// use pane_core::vm::Dead;
/// ```
#[derive(Debug)]
pub struct Dead;
impl VmState for Dead {}
impl private::Sealed for Dead {}

/// Represents the underlying VM execution engine.
///
/// # Example
/// ```no_run
/// use pane_core::vm::VmBackend;
/// // Handled internally by Vm
/// ```
pub enum VmBackend {
    /// C-based Native/QEMU KVM backend
    Native(SafeVm),
    /// AWS Firecracker MicroVM process backend
    Firecracker(Box<FirecrackerVm>),
}

/// Represents a Virtual Machine managed via the typestate pattern.
///
/// # Example
/// ```no_run
/// # tokio_test::block_on(async {
/// use pane_core::vm::Vm;
///
/// // Create a new Firecracker VM in the Spawning state
/// let mut vm = Vm::new_firecracker("example-vm");
/// vm.spawn().await.unwrap();
/// # });
/// ```
pub struct Vm<State: VmState> {
    id: String,
    backend: VmBackend,
    vsock_cid: u32,
    _state: PhantomData<State>,
}

impl<State: VmState> Vm<State> {
    /// Returns the unique identifier of the VM.
    ///
    /// # Example
    /// ```no_run
    /// use pane_core::vm::Vm;
    /// let vm = Vm::new_firecracker("vm-123");
    /// assert_eq!(vm.id(), "vm-123");
    /// ```
    pub fn id(&self) -> &str {
        &self.id
    }

    /// Returns the vsock Context Identifier (CID) for this VM.
    ///
    /// # Example
    /// ```no_run
    /// use pane_core::vm::Vm;
    /// let vm = Vm::new_firecracker("vm-123");
    /// assert_eq!(vm.vsock_cid(), 3);
    /// ```
    pub fn vsock_cid(&self) -> u32 {
        self.vsock_cid
    }

    /// Returns a reference to the underlying VM backend.
    ///
    /// # Example
    /// ```no_run
    /// use pane_core::vm::Vm;
    /// let vm = Vm::new_firecracker("vm-123");
    /// let backend = vm.backend();
    /// ```
    pub fn backend(&self) -> &VmBackend {
        &self.backend
    }

    /// Returns the process ID (PID) of the underlying VM backend if it is running.
    pub fn pid(&self) -> Option<u32> {
        match &self.backend {
            VmBackend::Firecracker(fc) => fc.pid(),
            VmBackend::Native(safe_vm) => {
                let raw_pid = safe_vm.get_pid();
                if raw_pid > 0 {
                    Some(raw_pid as u32)
                } else {
                    None
                }
            }
        }
    }

    /// Automatically registers the VM's process PID with its dedicated cgroup if running.
    fn ensure_cgroup_attached(&self) -> Result<()> {
        if let Some(pid) = self.pid() {
            if pid != std::process::id() {
                let cg = crate::resources::CgroupManager::create(&self.id)?;
                cg.attach_pid(pid)?;
            }
        }
        Ok(())
    }

    /// Applies resource controls to this VM's cgroup limits.
    pub fn apply_resources(&self, resources: &crate::resources::ResourceControls) -> Result<()> {
        let cg = crate::resources::CgroupManager::create(&self.id)?;
        cg.apply(resources)?;
        Ok(())
    }

    /// Helper to cleanup underlying VM resources (killing processes, removing sockets).
    async fn cleanup(&mut self) -> Result<()> {
        use tokio::io::{AsyncReadExt, AsyncWriteExt};

        match &mut self.backend {
            VmBackend::Firecracker(fc) => {
                fc.kill().await?;
            }
            VmBackend::Native(_) => {
                // SafeVm auto-destroys on drop.
            }
        }

        // Clean up QMP socket if it exists for QEMU
        let qmp_socket = if std::path::Path::new("/run/pane").join(format!("qmp-{}.sock", self.id)).exists() {
            Some(std::path::Path::new("/run/pane").join(format!("qmp-{}.sock", self.id)))
        } else if std::path::Path::new("/tmp/pane").join(format!("qmp-{}.sock", self.id)).exists() {
            Some(std::path::Path::new("/tmp/pane").join(format!("qmp-{}.sock", self.id)))
        } else {
            None
        };

        if let Some(sock_path) = qmp_socket {
            if let Ok(mut stream) = tokio::net::UnixStream::connect(&sock_path).await {
                let mut buf = [0u8; 1024];
                let _ = stream.read(&mut buf).await;
                let _ = stream.write_all(b"{\"execute\":\"qmp_capabilities\"}\n").await;
                let _ = stream.read(&mut buf).await;
                let _ = stream.write_all(b"{\"execute\":\"quit\"}\n").await;
                tokio::time::sleep(std::time::Duration::from_millis(50)).await;
            }
            let _ = std::fs::remove_file(sock_path);
        }

        // Clean up cgroup directory
        if let Ok(cg) = crate::resources::CgroupManager::create(&self.id) {
            let _ = cg.destroy();
        }
        Ok(())
    }

    /// Resolves the vsock UDS socket path for this VM.
    pub fn get_vsock_socket_path(&self) -> std::path::PathBuf {
        let run_pane = std::path::Path::new("/run/pane");
        if run_pane.exists() || std::fs::create_dir_all(run_pane).is_ok() {
            run_pane.join(format!("fc-vsock-{}.sock", self.id))
        } else {
            let tmp_pane = std::path::Path::new("/tmp/pane");
            let _ = std::fs::create_dir_all(tmp_pane);
            tmp_pane.join(format!("fc-vsock-{}.sock", self.id))
        }
    }
}

impl Vm<Spawning> {
    /// Creates a new Firecracker-backed VM in the Spawning state.
    ///
    /// # Example
    /// ```no_run
    /// use pane_core::vm::Vm;
    /// let vm = Vm::new_firecracker("fc-vm");
    /// ```
    pub fn new_firecracker(id: &str) -> Self {
        Self {
            id: id.to_string(),
            backend: VmBackend::Firecracker(Box::new(FirecrackerVm::new(id))),
            vsock_cid: 3, // Default CID
            _state: PhantomData,
        }
    }

    /// Creates a new Native KVM/QEMU-backed VM in the Spawning state.
    ///
    /// # Example
    /// ```no_run
    /// use pane_core::{Vm, SafeVm};
    /// # match SafeVm::create() {
    /// #    Ok(safe_vm) => {
    /// let vm = Vm::new_native("native-vm", safe_vm);
    /// #    }
    /// #    _ => {}
    /// # }
    /// ```
    pub fn new_native(id: &str, safe_vm: SafeVm) -> Self {
        Self {
            id: id.to_string(),
            backend: VmBackend::Native(safe_vm),
            vsock_cid: 3, // Default CID
            _state: PhantomData,
        }
    }

    /// Spawns the underlying VM processes (such as the Firecracker API listener).
    ///
    /// # Example
    /// ```no_run
    /// # tokio_test::block_on(async {
    /// use pane_core::vm::Vm;
    /// let mut vm = Vm::new_firecracker("fc-vm");
    /// vm.spawn().await.unwrap();
    /// # });
    /// ```
    pub async fn spawn(&mut self) -> Result<()> {
        match &mut self.backend {
            VmBackend::Firecracker(fc) => {
                fc.spawn().await?;
                self.ensure_cgroup_attached()?;
                Ok(())
            }
            VmBackend::Native(_) => Ok(()),
        }
    }

    /// Configures the machine's virtual CPU and memory limits.
    ///
    /// # Example
    /// ```no_run
    /// # tokio_test::block_on(async {
    /// use pane_core::vm::Vm;
    /// use pane_core::backends::MachineConfig;
    /// let mut vm = Vm::new_firecracker("fc-vm");
    /// vm.spawn().await.unwrap();
    /// vm.configure_machine(&MachineConfig {
    ///     vcpu_count: 2,
    ///     mem_size_mib: 1024,
    ///     smt: Some(false),
    ///     track_dirty_pages: Some(true),
    /// }).await.unwrap();
    /// # });
    /// ```
    pub async fn configure_machine(&self, config: &crate::backends::MachineConfig) -> Result<()> {
        match &self.backend {
            VmBackend::Firecracker(fc) => fc.configure_machine(config).await,
            VmBackend::Native(_) => Ok(()),
        }
    }

    /// Configures the vsock device for the VM.
    ///
    /// # Example
    /// ```no_run
    /// # tokio_test::block_on(async {
    /// use pane_core::vm::Vm;
    /// let mut vm = Vm::new_firecracker("fc-vm");
    /// vm.spawn().await.unwrap();
    /// vm.configure_vsock(4).await.unwrap();
    /// # });
    /// ```
    pub async fn configure_vsock(&mut self, guest_cid: u32) -> Result<()> {
        self.vsock_cid = guest_cid;
        match &self.backend {
            VmBackend::Firecracker(fc) => {
                let uds_path = self.get_vsock_socket_path();
                let config = crate::backends::VsockConfig {
                    vsock_id: "vsock0".to_string(),
                    guest_cid,
                    uds_path: uds_path.to_string_lossy().into_owned(),
                };
                fc.configure_vsock(&config).await?;
            }
            VmBackend::Native(_) => {}
        }
        Ok(())
    }

    /// Configures a network interface for the VM.
    pub async fn configure_network_interface(
        &self,
        config: &crate::backends::NetworkInterfaceConfig,
    ) -> Result<()> {
        match &self.backend {
            VmBackend::Firecracker(fc) => fc.configure_network_interface(config).await,
            VmBackend::Native(_) => Ok(()),
        }
    }

    /// Configures the kernel boot source details.
    ///
    /// # Example
    /// ```no_run
    /// # tokio_test::block_on(async {
    /// use pane_core::vm::Vm;
    /// use pane_core::backends::BootSource;
    /// let mut vm = Vm::new_firecracker("fc-vm");
    /// vm.spawn().await.unwrap();
    /// vm.configure_boot_source(&BootSource {
    ///     kernel_image_path: "/path/to/vmlinux".to_string(),
    ///     boot_args: Some("console=ttyS0 reboot=k panic=1 pci=off".to_string()),
    /// }).await.unwrap();
    /// # });
    /// ```
    pub async fn configure_boot_source(&self, boot: &crate::backends::BootSource) -> Result<()> {
        match &self.backend {
            VmBackend::Firecracker(fc) => fc.configure_boot_source(boot).await,
            VmBackend::Native(_) => Ok(()),
        }
    }

    /// Configures host files backing the storage drives of the VM.
    ///
    /// # Example
    /// ```no_run
    /// # tokio_test::block_on(async {
    /// use pane_core::vm::Vm;
    /// use pane_core::backends::Drive;
    /// let mut vm = Vm::new_firecracker("fc-vm");
    /// vm.spawn().await.unwrap();
    /// vm.configure_drive(&Drive {
    ///     drive_id: "rootfs".to_string(),
    ///     path_on_host: "/path/to/rootfs.img".to_string(),
    ///     is_root_device: true,
    ///     is_read_only: false,
    /// }).await.unwrap();
    /// # });
    /// ```
    pub async fn configure_drive(&self, drive: &crate::backends::Drive) -> Result<()> {
        match &self.backend {
            VmBackend::Firecracker(fc) => fc.configure_drive(drive).await,
            VmBackend::Native(_) => Ok(()),
        }
    }

    /// Loads a snapshot directly into the VM instead of executing a standard kernel boot.
    ///
    /// # Example
    /// ```no_run
    /// # tokio_test::block_on(async {
    /// use pane_core::vm::Vm;
    /// let mut vm = Vm::new_firecracker("fc-vm");
    /// vm.spawn().await.unwrap();
    /// vm.load_snapshot("/path/to/snap", "/path/to/mem").await.unwrap();
    /// # });
    /// ```
    pub async fn load_snapshot(&self, snapshot_path: &str, mem_file_path: &str) -> Result<()> {
        match &self.backend {
            VmBackend::Firecracker(fc) => fc.load_snapshot(snapshot_path, mem_file_path).await,
            VmBackend::Native(_) => Ok(()),
        }
    }

    /// Creates a new VM by forking an existing snapshot, returning a VM in the Frozen state.
    ///
    /// # Example
    /// ```no_run
    /// # tokio_test::block_on(async {
    /// use pane_core::vm::Vm;
    /// let frozen_vm = Vm::fork_firecracker("fc-fork-1", "/path/to/snap", "/path/to/mem").await.unwrap();
    /// # });
    /// ```
    pub async fn fork_firecracker(
        id: &str,
        snapshot_path: &str,
        mem_file_path: &str,
    ) -> Result<Vm<Frozen>> {
        let mut vm = Self::new_firecracker(id);
        vm.spawn().await?;
        vm.load_snapshot(snapshot_path, mem_file_path).await?;
        Ok(Vm {
            id: vm.id,
            backend: vm.backend,
            vsock_cid: vm.vsock_cid,
            _state: PhantomData,
        })
    }

    /// Transitions the Spawning VM to Running by starting execution.
    ///
    /// # Example
    /// ```no_run
    /// # tokio_test::block_on(async {
    /// use pane_core::vm::Vm;
    /// let mut vm = Vm::new_firecracker("fc-vm");
    /// vm.spawn().await.unwrap();
    /// // ... configure machine, boot source, drives ...
    /// let running_vm = vm.start().await.unwrap();
    /// # });
    /// ```
    pub async fn start(self) -> Result<Vm<Running>> {
        match &self.backend {
            VmBackend::Firecracker(fc) => fc.start().await?,
            VmBackend::Native(_) => {}
        }
        let running_vm = Vm {
            id: self.id,
            backend: self.backend,
            vsock_cid: self.vsock_cid,
            _state: PhantomData,
        };
        running_vm.ensure_cgroup_attached()?;
        Ok(running_vm)
    }

    /// Instantly destroys the Spawning VM and transitions to Dead.
    ///
    /// # Example
    /// ```no_run
    /// # tokio_test::block_on(async {
    /// use pane_core::vm::Vm;
    /// let mut vm = Vm::new_firecracker("fc-vm");
    /// let dead_vm = vm.destroy().await.unwrap();
    /// # });
    /// ```
    pub async fn destroy(mut self) -> Result<Vm<Dead>> {
        self.cleanup().await?;
        Ok(Vm {
            id: self.id,
            backend: self.backend,
            vsock_cid: self.vsock_cid,
            _state: PhantomData,
        })
    }
}

impl Vm<Running> {
    /// Suspends VM execution and transitions it to the Frozen state.
    ///
    /// # Example
    /// ```no_run
    /// # tokio_test::block_on(async {
    /// use pane_core::vm::Vm;
    /// # let mut vm = Vm::new_firecracker("fc-vm");
    /// # let running_vm = vm.start().await.unwrap();
    /// let frozen_vm = running_vm.freeze().await.unwrap();
    /// # });
    /// ```
    pub async fn freeze(self) -> Result<Vm<Frozen>> {
        if is_qemu(&self.id) {
            send_qmp_command_on_demand(&self.id, "{\"execute\":\"stop\"}").await?;
        } else {
            match &self.backend {
                VmBackend::Firecracker(fc) => fc.pause().await?,
                VmBackend::Native(vm) => {
                    if vm.get_backend() == crate::ffi::pane_backend_t::Qemu {
                        vm.qemu_suspend()?;
                    }
                }
            }
        }
        Ok(Vm {
            id: self.id,
            backend: self.backend,
            vsock_cid: self.vsock_cid,
            _state: PhantomData,
        })
    }

    /// Instantly terminates the running VM, freeing resources and transitioning to Dead.
    ///
    /// # Example
    /// ```no_run
    /// # tokio_test::block_on(async {
    /// use pane_core::vm::Vm;
    /// # let mut vm = Vm::new_firecracker("fc-vm");
    /// # let running_vm = vm.start().await.unwrap();
    /// let dead_vm = running_vm.destroy().await.unwrap();
    /// # });
    /// ```
    pub async fn destroy(mut self) -> Result<Vm<Dead>> {
        self.cleanup().await?;
        Ok(Vm {
            id: self.id,
            backend: self.backend,
            vsock_cid: self.vsock_cid,
            _state: PhantomData,
        })
    }

    /// Executes a command inside the running VM via vsock.
    ///
    /// # Example
    /// ```no_run
    /// # tokio_test::block_on(async {
    /// use pane_core::vm::Vm;
    /// use pane_core::exec::ExecRequest;
    /// # let mut vm = Vm::new_firecracker("fc-vm");
    /// # let running_vm = vm.start().await.unwrap();
    /// let req = ExecRequest {
    ///     command: "/bin/ls".to_string(),
    ///     args: vec!["-l".to_string()],
    /// };
    /// let mut stream = running_vm.exec(&req).await.unwrap();
    /// # });
    /// ```
    pub async fn exec(
        &self,
        req: &crate::exec::ExecRequest,
    ) -> Result<crate::exec::ExecStream<tokio::net::UnixStream>> {
        let vsock_uds_path = self.get_vsock_socket_path();
        match &self.backend {
            VmBackend::Firecracker(_) => {
                crate::exec::exec_in_guest(&vsock_uds_path, req, true).await
            }
            VmBackend::Native(_) => crate::exec::exec_in_guest(&vsock_uds_path, req, false).await,
        }
    }

    /// Unsafely reconstructs a Running VM instance from an ID.
    pub fn assume_running(id: &str) -> Self {
        Self {
            id: id.to_string(),
            backend: VmBackend::Firecracker(Box::new(FirecrackerVm::new(id))),
            vsock_cid: 3,
            _state: PhantomData,
        }
    }
}

impl Vm<Frozen> {
    /// Resumes VM execution from the Frozen state, transitioning it to Running.
    ///
    /// # Example
    /// ```no_run
    /// # tokio_test::block_on(async {
    /// use pane_core::vm::Vm;
    /// # let mut vm = Vm::new_firecracker("fc-vm");
    /// # let running_vm = vm.start().await.unwrap();
    /// # let frozen_vm = running_vm.freeze().await.unwrap();
    /// let running_vm = frozen_vm.resume().await.unwrap();
    /// # });
    /// ```
    pub async fn resume(self) -> Result<Vm<Running>> {
        if is_qemu(&self.id) {
            send_qmp_command_on_demand(&self.id, "{\"execute\":\"cont\"}").await?;
        } else {
            match &self.backend {
                VmBackend::Firecracker(fc) => fc.resume().await?,
                VmBackend::Native(vm) => {
                    if vm.get_backend() == crate::ffi::pane_backend_t::Qemu {
                        vm.qemu_resume()?;
                    }
                }
            }
        }
        Ok(Vm {
            id: self.id,
            backend: self.backend,
            vsock_cid: self.vsock_cid,
            _state: PhantomData,
        })
    }

    /// Patches an existing block device in a frozen VM (e.g., after loading a snapshot for a fork).
    pub async fn patch_drive(&self, drive_id: &str, path_on_host: &str) -> Result<()> {
        match &self.backend {
            VmBackend::Firecracker(fc) => fc.patch_drive(drive_id, path_on_host).await,
            VmBackend::Native(_) => Ok(()),
        }
    }

    /// Updates the vsock configuration in a frozen VM (e.g., after loading a snapshot for a fork).
    pub async fn configure_vsock(&mut self, guest_cid: u32) -> Result<()> {
        self.vsock_cid = guest_cid;
        match &self.backend {
            VmBackend::Firecracker(fc) => {
                let uds_path = self.get_vsock_socket_path();
                let config = crate::backends::VsockConfig {
                    vsock_id: "vsock0".to_string(),
                    guest_cid,
                    uds_path: uds_path.to_string_lossy().into_owned(),
                };
                fc.configure_vsock(&config).await?;
            }
            VmBackend::Native(_) => {}
        }
        Ok(())
    }

    /// Configures a network interface in a frozen VM (e.g., after loading a snapshot for a fork).
    pub async fn configure_network_interface(
        &self,
        config: &crate::backends::NetworkInterfaceConfig,
    ) -> Result<()> {
        match &self.backend {
            VmBackend::Firecracker(fc) => fc.configure_network_interface(config).await,
            VmBackend::Native(_) => Ok(()),
        }
    }

    /// Creates a snapshot of the Frozen VM to the specified file paths.
    ///
    /// # Example
    /// ```no_run
    /// # tokio_test::block_on(async {
    /// use pane_core::vm::Vm;
    /// # let mut vm = Vm::new_firecracker("fc-vm");
    /// # let running_vm = vm.start().await.unwrap();
    /// # let frozen_vm = running_vm.freeze().await.unwrap();
    /// frozen_vm.create_snapshot("/path/to/snap", "/path/to/mem").await.unwrap();
    /// # });
    /// ```
    pub async fn create_snapshot(&self, snapshot_path: &str, mem_file_path: &str) -> Result<()> {
        if is_qemu(&self.id) {
            let cmd = format!(
                "{{\"execute\":\"migrate\",\"arguments\":{{\"uri\":\"exec:cat > {}\"}}}}",
                mem_file_path
            );
            send_qmp_command_on_demand(&self.id, &cmd).await?;

            // Wait for migration to complete
            let start = std::time::Instant::now();
            let timeout = std::time::Duration::from_secs(15);
            while start.elapsed() < timeout {
                tokio::time::sleep(std::time::Duration::from_millis(100)).await;
                if let Ok(resp) = send_qmp_command_on_demand(&self.id, "{\"execute\":\"query-migrate\"}").await {
                    if resp.contains("\"status\": \"completed\"") {
                        break;
                    }
                    if resp.contains("\"status\": \"failed\"") || resp.contains("\"status\": \"cancelled\"") {
                        return Err(crate::error::PaneError::Api {
                            status: "Migration failed".to_string(),
                            body: resp,
                        });
                    }
                }
            }
        } else {
            match &self.backend {
                VmBackend::Firecracker(fc) => fc.create_snapshot(snapshot_path, mem_file_path).await?,
                VmBackend::Native(_) => {
                    // Native KVM snapshotting
                }
            }
        }
        Ok(())
    }

    /// Instantly terminates the frozen VM, freeing resources and transitioning to Dead.
    ///
    /// # Example
    /// ```no_run
    /// # tokio_test::block_on(async {
    /// use pane_core::vm::Vm;
    /// # let mut vm = Vm::new_firecracker("fc-vm");
    /// # let running_vm = vm.start().await.unwrap();
    /// # let frozen_vm = running_vm.freeze().await.unwrap();
    /// let dead_vm = frozen_vm.destroy().await.unwrap();
    /// # });
    /// ```
    pub async fn destroy(mut self) -> Result<Vm<Dead>> {
        self.cleanup().await?;
        Ok(Vm {
            id: self.id,
            backend: self.backend,
            vsock_cid: self.vsock_cid,
            _state: PhantomData,
        })
    }

    /// Unsafely reconstructs a Frozen VM instance from an ID.
    pub fn assume_frozen(id: &str) -> Self {
        Self {
            id: id.to_string(),
            backend: VmBackend::Firecracker(Box::new(FirecrackerVm::new(id))),
            vsock_cid: 3,
            _state: PhantomData,
        }
    }
}

fn is_qemu(id: &str) -> bool {
    std::path::Path::new("/run/pane").join(format!("qmp-{}.sock", id)).exists() ||
    std::path::Path::new("/tmp/pane").join(format!("qmp-{}.sock", id)).exists()
}

async fn send_qmp_command_on_demand(id: &str, cmd: &str) -> Result<String> {
    use tokio::io::{AsyncReadExt, AsyncWriteExt};
    let run_pane = std::path::Path::new("/run/pane");
    let qmp_socket = if run_pane.join(format!("qmp-{}.sock", id)).exists() {
        run_pane.join(format!("qmp-{}.sock", id))
    } else {
        std::path::Path::new("/tmp/pane").join(format!("qmp-{}.sock", id))
    };

    let mut stream = tokio::net::UnixStream::connect(&qmp_socket).await.map_err(|e| {
        crate::error::PaneError::Socket(format!("QMP socket connect failed: {}", e))
    })?;

    let mut buf = [0u8; 1024];
    let _ = stream.read(&mut buf).await;
    let _ = stream.write_all(b"{\"execute\":\"qmp_capabilities\"}\n").await;
    let _ = stream.read(&mut buf).await;
    let _ = stream.write_all(format!("{}\n", cmd).as_bytes()).await;
    let n = stream.read(&mut buf).await.map_err(|e| {
        crate::error::PaneError::Socket(format!("QMP read failed: {}", e))
    })?;

    Ok(String::from_utf8_lossy(&buf[..n]).into_owned())
}
