use std::marker::PhantomData;
use crate::error::Result;
use crate::ffi::SafeVm;
use crate::backends::FirecrackerVm;

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

    /// Helper to cleanup underlying VM resources (killing processes, removing sockets).
    async fn cleanup(&mut self) -> Result<()> {
        match &mut self.backend {
            VmBackend::Firecracker(fc) => {
                fc.kill().await?;
            }
            VmBackend::Native(_) => {
                // SafeVm auto-destroys on drop.
            }
        }
        Ok(())
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
            VmBackend::Firecracker(fc) => fc.spawn().await,
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
        Ok(Vm {
            id: self.id,
            backend: self.backend,
            _state: PhantomData,
        })
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
        match &self.backend {
            VmBackend::Firecracker(fc) => fc.pause().await?,
            VmBackend::Native(vm) => {
                if vm.get_backend() == crate::ffi::pane_backend_t::Qemu {
                    vm.qemu_suspend()?;
                }
            }
        }
        Ok(Vm {
            id: self.id,
            backend: self.backend,
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
            _state: PhantomData,
        })
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
        match &self.backend {
            VmBackend::Firecracker(fc) => fc.resume().await?,
            VmBackend::Native(vm) => {
                if vm.get_backend() == crate::ffi::pane_backend_t::Qemu {
                    vm.qemu_resume()?;
                }
            }
        }
        Ok(Vm {
            id: self.id,
            backend: self.backend,
            _state: PhantomData,
        })
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
        match &self.backend {
            VmBackend::Firecracker(fc) => fc.create_snapshot(snapshot_path, mem_file_path).await?,
            VmBackend::Native(_) => {
                // Native KVM snapshotting
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
            _state: PhantomData,
        })
    }
}
