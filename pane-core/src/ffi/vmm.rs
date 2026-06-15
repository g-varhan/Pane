// SPDX-License-Identifier: Apache-2.0

use crate::error::{check_ffi, Result};
use std::ffi::CString;

/// Opaque pointer corresponding to `pane_vm_t` in C.
#[repr(C)]
pub struct pane_vm {
    _unused: [u8; 0],
}

/// VM Backend Types mapped to C enum `pane_backend_t`.
#[repr(C)]
#[derive(Debug, Copy, Clone, PartialEq, Eq)]
pub enum pane_backend_t {
    Native = 0,
    Qemu = 1,
}

#[repr(C)]
#[derive(Debug, Clone)]
pub struct pane_vmm_config_t {
    pub vm_id: *const libc::c_char,
    pub vmm_type: *const libc::c_char,
    pub vcpus: u32,
    pub memory_bytes: u64,
    pub disk_path: *const libc::c_char,
    pub disk_format: *const libc::c_char,
    pub virtio_net: bool,
    pub virtio_blk: bool,
    pub virtio_rng: bool,
    pub net_bridge: *const libc::c_char,
    pub kernel_path: *const libc::c_char,
    pub cmdline: *const libc::c_char,
    pub extra_args: *const *const libc::c_char,
}

#[repr(C)]
#[derive(Debug, Copy, Clone, Default)]
pub struct kvm_regs {
    pub rax: u64,
    pub rbx: u64,
    pub rcx: u64,
    pub rdx: u64,
    pub rsi: u64,
    pub rdi: u64,
    pub rsp: u64,
    pub rbp: u64,
    pub r8: u64,
    pub r9: u64,
    pub r10: u64,
    pub r11: u64,
    pub r12: u64,
    pub r13: u64,
    pub r14: u64,
    pub r15: u64,
    pub rip: u64,
    pub rflags: u64,
}

#[repr(C)]
#[derive(Debug, Copy, Clone, Default)]
pub struct kvm_segment {
    pub base: u64,
    pub limit: u32,
    pub selector: u16,
    pub type_: u8,
    pub present: u8,
    pub dpl: u8,
    pub db: u8,
    pub s: u8,
    pub l: u8,
    pub g: u8,
    pub avl: u8,
    pub unusable: u8,
    pub padding: u8,
}

#[repr(C)]
#[derive(Debug, Copy, Clone, Default)]
pub struct kvm_dtable {
    pub base: u64,
    pub limit: u16,
    pub padding: [u16; 3],
}

#[repr(C)]
#[derive(Debug, Copy, Clone, Default)]
pub struct kvm_sregs {
    pub cs: kvm_segment,
    pub ds: kvm_segment,
    pub es: kvm_segment,
    pub fs: kvm_segment,
    pub gs: kvm_segment,
    pub ss: kvm_segment,
    pub tr: kvm_segment,
    pub ldt: kvm_segment,
    pub gdt: kvm_dtable,
    pub idt: kvm_dtable,
    pub cr0: u64,
    pub cr2: u64,
    pub cr3: u64,
    pub cr4: u64,
    pub cr8: u64,
    pub efer: u64,
    pub apic_base: u64,
    pub interrupt_bitmap: [u64; 4],
}

extern "C" {
    fn pane_vm_create(vm_out: *mut *mut pane_vm) -> libc::c_int;
    fn pane_vm_destroy(vm: *mut pane_vm);
    fn pane_vm_set_user_memory_region(
        vm: *mut pane_vm,
        slot: u32,
        guest_phys_addr: u64,
        memory_size: u64,
        userspace_addr: u64,
        flags: u32,
    ) -> libc::c_int;
    fn pane_vm_get_kvm_fd(vm: *const pane_vm) -> libc::c_int;
    fn pane_vm_get_vm_fd(vm: *const pane_vm) -> libc::c_int;
    fn pane_vm_init_irqchip(vm: *mut pane_vm) -> libc::c_int;
    fn pane_vm_vcpu_create(vm: *mut pane_vm, vcpu_id: u32) -> libc::c_int;
    fn pane_vm_vcpu_set_regs(vm: *mut pane_vm, vcpu_id: u32, regs: *const kvm_regs) -> libc::c_int;
    fn pane_vm_vcpu_get_regs(vm: *const pane_vm, vcpu_id: u32, regs: *mut kvm_regs) -> libc::c_int;
    fn pane_vm_vcpu_set_sregs(
        vm: *mut pane_vm,
        vcpu_id: u32,
        sregs: *const kvm_sregs,
    ) -> libc::c_int;
    fn pane_vm_vcpu_get_sregs(
        vm: *const pane_vm,
        vcpu_id: u32,
        sregs: *mut kvm_sregs,
    ) -> libc::c_int;
    fn pane_vm_vcpu_run(vm: *mut pane_vm, vcpu_id: u32) -> libc::c_int;
    fn pane_vm_get_vcpu_fd(vm: *const pane_vm, vcpu_id: u32) -> libc::c_int;
    fn pane_vm_setup_firecracker_mode(
        vm: *mut pane_vm,
        vcpu_id: u32,
        entry_point: u64,
    ) -> libc::c_int;
    fn pane_vm_setup_virtio_mmio(
        vm: *mut pane_vm,
        base_addr: u64,
        size: u64,
        irq: libc::c_int,
    ) -> libc::c_int;
    fn pane_vm_setup_virtio_blk(
        vm: *mut pane_vm,
        base_addr: u64,
        size: u64,
        irq: libc::c_int,
        disk_path: *const libc::c_char,
    ) -> libc::c_int;
    fn pane_vm_set_virtio_console(vm: *mut pane_vm) -> libc::c_int;
    fn pane_vm_get_backend(vm: *const pane_vm) -> pane_backend_t;
    fn pane_vm_setup_qemu_mode(
        vm: *mut pane_vm,
        config: *const pane_vmm_config_t,
        qmp_socket_path: *const libc::c_char,
    ) -> libc::c_int;
    fn pane_vm_qemu_suspend(vm: *mut pane_vm) -> libc::c_int;
    fn pane_vm_qemu_resume(vm: *mut pane_vm) -> libc::c_int;
    fn pane_vm_qemu_query_status(
        vm: *mut pane_vm,
        status_out: *mut libc::c_char,
        max_len: libc::size_t,
    ) -> libc::c_int;
    fn pane_vm_get_pid(vm: *const pane_vm) -> libc::c_int;
    fn pane_vm_reconstruct_qemu(
        vm_out: *mut *mut pane_vm,
        vm_id: *const libc::c_char,
        qemu_pid: libc::c_int,
        qmp_socket_path: *const libc::c_char,
    ) -> libc::c_int;
}

/// A safe, typed Rust wrapper managing the raw `pane_vm_t` structure lifecycle.
pub struct SafeVm {
    raw: *mut pane_vm,
}

unsafe impl Send for SafeVm {}
unsafe impl Sync for SafeVm {}

impl SafeVm {
    /// Creates a new VM instance.
    pub fn create() -> Result<Self> {
        let mut raw = std::ptr::null_mut();
        // SAFETY: pane_vm_create writes a valid pointer to raw on success.
        let ret = unsafe { pane_vm_create(&mut raw) };
        check_ffi(ret, "Create VM")?;
        if raw.is_null() {
            return Err(crate::error::PaneError::Vmm(
                libc::ENOMEM,
                "pane_vm_create returned null pointer without error code".to_string(),
            ));
        }
        Ok(Self { raw })
    }

    /// Sets user memory region for the VM.
    pub fn set_user_memory_region(
        &self,
        slot: u32,
        guest_phys_addr: u64,
        memory_size: u64,
        userspace_addr: u64,
        flags: u32,
    ) -> Result<()> {
        // SAFETY: The raw pointer is valid for the lifetime of self.
        let ret = unsafe {
            pane_vm_set_user_memory_region(
                self.raw,
                slot,
                guest_phys_addr,
                memory_size,
                userspace_addr,
                flags,
            )
        };
        check_ffi(ret, "Set user memory region")?;
        Ok(())
    }

    /// Gets the KVM file descriptor associated with the VM.
    pub fn get_kvm_fd(&self) -> i32 {
        // SAFETY: The raw pointer is valid for the lifetime of self.
        unsafe { pane_vm_get_kvm_fd(self.raw) }
    }

    /// Gets the VM file descriptor associated with the VM.
    pub fn get_vm_fd(&self) -> i32 {
        // SAFETY: The raw pointer is valid for the lifetime of self.
        unsafe { pane_vm_get_vm_fd(self.raw) }
    }

    /// Initializes the in-kernel IRQ chip.
    pub fn init_irqchip(&self) -> Result<()> {
        // SAFETY: The raw pointer is valid for the lifetime of self.
        let ret = unsafe { pane_vm_init_irqchip(self.raw) };
        check_ffi(ret, "Initialize IRQ chip")?;
        Ok(())
    }

    /// Creates a vCPU with the specified ID.
    pub fn vcpu_create(&self, vcpu_id: u32) -> Result<()> {
        // SAFETY: The raw pointer is valid for the lifetime of self.
        let ret = unsafe { pane_vm_vcpu_create(self.raw, vcpu_id) };
        check_ffi(ret, "Create vCPU")?;
        Ok(())
    }

    /// Sets general purpose registers for a vCPU.
    pub fn vcpu_set_regs(&self, vcpu_id: u32, regs: &kvm_regs) -> Result<()> {
        // SAFETY: The raw pointer is valid, and regs is a valid reference.
        let ret = unsafe { pane_vm_vcpu_set_regs(self.raw, vcpu_id, regs) };
        check_ffi(ret, "Set vCPU registers")?;
        Ok(())
    }

    /// Gets general purpose registers for a vCPU.
    pub fn vcpu_get_regs(&self, vcpu_id: u32) -> Result<kvm_regs> {
        // SAFETY: kvm_regs is a plain old data struct (integer fields only), so zeroing it is safe and does not produce invalid bit patterns.
        let mut regs: kvm_regs = unsafe { std::mem::zeroed() };
        // SAFETY: The raw pointer is valid, and regs pointer is valid.
        let ret = unsafe { pane_vm_vcpu_get_regs(self.raw, vcpu_id, &mut regs) };
        check_ffi(ret, "Get vCPU registers")?;
        Ok(regs)
    }

    /// Sets special/segment registers for a vCPU.
    pub fn vcpu_set_sregs(&self, vcpu_id: u32, sregs: &kvm_sregs) -> Result<()> {
        // SAFETY: The raw pointer is valid, and sregs is a valid reference.
        let ret = unsafe { pane_vm_vcpu_set_sregs(self.raw, vcpu_id, sregs) };
        check_ffi(ret, "Set vCPU segment registers")?;
        Ok(())
    }

    /// Gets special/segment registers for a vCPU.
    pub fn vcpu_get_sregs(&self, vcpu_id: u32) -> Result<kvm_sregs> {
        // SAFETY: kvm_sregs is a plain old data struct (integer fields only), so zeroing it is safe and does not produce invalid bit patterns.
        let mut sregs: kvm_sregs = unsafe { std::mem::zeroed() };
        // SAFETY: The raw pointer is valid, and sregs pointer is valid.
        let ret = unsafe { pane_vm_vcpu_get_sregs(self.raw, vcpu_id, &mut sregs) };
        check_ffi(ret, "Get vCPU segment registers")?;
        Ok(sregs)
    }

    /// Runs a vCPU.
    pub fn vcpu_run(&self, vcpu_id: u32) -> Result<()> {
        // SAFETY: The raw pointer is valid for the lifetime of self.
        let ret = unsafe { pane_vm_vcpu_run(self.raw, vcpu_id) };
        check_ffi(ret, "Run vCPU")?;
        Ok(())
    }

    /// Gets the vCPU file descriptor.
    pub fn get_vcpu_fd(&self, vcpu_id: u32) -> Result<i32> {
        // SAFETY: The raw pointer is valid.
        let ret = unsafe { pane_vm_get_vcpu_fd(self.raw, vcpu_id) };
        check_ffi(ret, "Get vCPU file descriptor")
    }

    /// Configures the VM for direct 64-bit boot (Firecracker Mode).
    pub fn setup_firecracker_mode(&self, vcpu_id: u32, entry_point: u64) -> Result<()> {
        // SAFETY: The raw pointer is valid.
        let ret = unsafe { pane_vm_setup_firecracker_mode(self.raw, vcpu_id, entry_point) };
        check_ffi(ret, "Set up Firecracker Mode")?;
        Ok(())
    }

    /// Sets up a virtio-mmio console device.
    pub fn setup_virtio_mmio(&self, base_addr: u64, size: u64, irq: i32) -> Result<()> {
        // SAFETY: The raw pointer is valid.
        let ret = unsafe { pane_vm_setup_virtio_mmio(self.raw, base_addr, size, irq) };
        check_ffi(ret, "Set up Virtio-MMIO console")?;
        Ok(())
    }

    /// Sets up a virtio-mmio block device back-ended by the specified file.
    pub fn setup_virtio_blk(
        &self,
        base_addr: u64,
        size: u64,
        irq: i32,
        disk_path: &str,
    ) -> Result<()> {
        let c_path = CString::new(disk_path)
            .map_err(|e| std::io::Error::new(std::io::ErrorKind::InvalidInput, e))?;
        // SAFETY: The raw pointer is valid and c_path is a null-terminated C string.
        let ret =
            unsafe { pane_vm_setup_virtio_blk(self.raw, base_addr, size, irq, c_path.as_ptr()) };
        check_ffi(ret, "Set up Virtio-MMIO block device")?;
        Ok(())
    }

    /// Sets up a virtio-serial console.
    pub fn set_virtio_console(&self) -> Result<()> {
        // SAFETY: The raw pointer is valid.
        let ret = unsafe { pane_vm_set_virtio_console(self.raw) };
        check_ffi(ret, "Set Virtio console")?;
        Ok(())
    }

    /// Gets current VM backend type.
    pub fn get_backend(&self) -> pane_backend_t {
        // SAFETY: The raw pointer is valid.
        unsafe { pane_vm_get_backend(self.raw) }
    }

    /// Configures the VM for QEMU mode, spawning QEMU with KVM acceleration and QMP.
    ///
    /// # Safety
    /// The caller must ensure that `config` points to a valid, fully initialized `pane_vmm_config_t` structure.
    pub unsafe fn setup_qemu_mode(
        &self,
        config: *const pane_vmm_config_t,
        qmp_socket_path: &str,
    ) -> Result<()> {
        let c_qmp = CString::new(qmp_socket_path)
            .map_err(|e| std::io::Error::new(std::io::ErrorKind::InvalidInput, e))?;
        // SAFETY: The raw pointer is valid, and C strings are null-terminated.
        let ret = unsafe { pane_vm_setup_qemu_mode(self.raw, config, c_qmp.as_ptr()) };
        check_ffi(ret, "Set up QEMU mode")?;
        Ok(())
    }

    /// Suspends a QEMU VM.
    pub fn qemu_suspend(&self) -> Result<()> {
        // SAFETY: The raw pointer is valid.
        let ret = unsafe { pane_vm_qemu_suspend(self.raw) };
        check_ffi(ret, "Suspend QEMU VM")?;
        Ok(())
    }

    /// Resumes a QEMU VM.
    pub fn qemu_resume(&self) -> Result<()> {
        // SAFETY: The raw pointer is valid.
        let ret = unsafe { pane_vm_qemu_resume(self.raw) };
        check_ffi(ret, "Resume QEMU VM")?;
        Ok(())
    }

    /// Queries the execution status of a QEMU VM.
    pub fn qemu_query_status(&self) -> Result<String> {
        let mut buf = vec![0u8; 256];
        // SAFETY: The raw pointer is valid, and buffer pointer is valid for buffer length.
        let ret = unsafe {
            pane_vm_qemu_query_status(self.raw, buf.as_mut_ptr() as *mut libc::c_char, buf.len())
        };
        check_ffi(ret, "Query QEMU status")?;
        // SAFETY: buf is populated and null-terminated by the native function on success.
        let c_str = unsafe { std::ffi::CStr::from_ptr(buf.as_ptr() as *const libc::c_char) };
        Ok(c_str.to_string_lossy().into_owned())
    }

    /// Gets the PID associated with the VM.
    pub fn get_pid(&self) -> i32 {
        // SAFETY: The raw pointer is valid.
        unsafe { pane_vm_get_pid(self.raw) }
    }

    /// Reconstructs a running QEMU VM structure.
    pub fn reconstruct_qemu(vm_id: &str, qemu_pid: i32, qmp_socket_path: &str) -> Result<Self> {
        let c_id = CString::new(vm_id)
            .map_err(|e| std::io::Error::new(std::io::ErrorKind::InvalidInput, e))?;
        let c_qmp = CString::new(qmp_socket_path)
            .map_err(|e| std::io::Error::new(std::io::ErrorKind::InvalidInput, e))?;
        let mut raw = std::ptr::null_mut();
        // SAFETY: pane_vm_reconstruct_qemu writes a valid pointer on success.
        let ret =
            unsafe { pane_vm_reconstruct_qemu(&mut raw, c_id.as_ptr(), qemu_pid, c_qmp.as_ptr()) };
        check_ffi(ret, "Reconstruct QEMU VM")?;
        if raw.is_null() {
            return Err(crate::error::PaneError::Vmm(
                libc::ENOMEM,
                "pane_vm_reconstruct_qemu returned null pointer".to_string(),
            ));
        }
        Ok(Self { raw })
    }
}

impl Drop for SafeVm {
    fn drop(&mut self) {
        if !self.raw.is_null() {
            // SAFETY: The raw pointer was successfully created, not freed before, and is owned by Self.
            unsafe { pane_vm_destroy(self.raw) };
        }
    }
}
