// SPDX-License-Identifier: Apache-2.0

use crate::error::{PaneError, Result};
use aya::maps::HashMap;
use aya::programs::{SchedClassifier, TcAttachType};
use aya::{include_bytes_aligned, Ebpf};
use once_cell::sync::Lazy;
use std::net::Ipv4Addr;
use std::sync::Mutex;

// Global eBPF instance
static BPF: Lazy<Mutex<Option<Ebpf>>> = Lazy::new(|| Mutex::new(None));

fn lock_bpf() -> Result<std::sync::MutexGuard<'static, Option<Ebpf>>> {
    BPF.lock()
        .map_err(|e| PaneError::Socket(format!("BPF mutex poisoned: {}", e)))
}

/// Initializes the eBPF program by loading the compiled ELF bytecode.
pub fn init_network_ebpf() -> Result<()> {
    let mut guard = lock_bpf()?;
    if guard.is_some() {
        return Ok(());
    }

    // Align and load the raw eBPF ELF bytes
    let bpf_bytes = include_bytes_aligned!(env!("BPF_OBJ_PATH"));
    let bpf = Ebpf::load(bpf_bytes)
        .map_err(|e| PaneError::Socket(format!("Failed to load eBPF bytecode: {}", e)))?;

    *guard = Some(bpf);
    Ok(())
}

/// Attaches the micro-segmentation TC filter to the specified host interface.
pub fn attach_filter_to_interface(iface: &str) -> Result<()> {
    init_network_ebpf()?;
    let mut guard = lock_bpf()?;
    if let Some(ref mut bpf) = *guard {
        // Retrieve SchedClassifier program
        let program: &mut SchedClassifier = bpf
            .program_mut("pane_filter")
            .ok_or_else(|| {
                PaneError::Socket("pane_filter program not found in BPF object".to_string())
            })?
            .try_into()
            .map_err(|e| {
                PaneError::Socket(format!(
                    "Failed to cast BPF program to SchedClassifier: {}",
                    e
                ))
            })?;

        // Load classifier into kernel
        program
            .load()
            .map_err(|e| PaneError::Socket(format!("Failed to load BPF program: {}", e)))?;

        // Attach to the interface on ingress
        program.attach(iface, TcAttachType::Ingress).map_err(|e| {
            PaneError::Socket(format!(
                "Failed to attach BPF program to interface {}: {}",
                iface, e
            ))
        })?;
    }
    Ok(())
}

/// Registers a VM IPv4 address with its micro-segmentation group in the BPF map.
pub fn register_vm_network_group(ip: Ipv4Addr, group_id: u32) -> Result<()> {
    init_network_ebpf()?;
    let mut guard = lock_bpf()?;
    if let Some(ref mut bpf) = *guard {
        // Retrieve ip_groups map
        let mut ip_groups: HashMap<_, u32, u32> =
            HashMap::try_from(bpf.map_mut("ip_groups").ok_or_else(|| {
                PaneError::Socket("ip_groups map not found in BPF object".to_string())
            })?)
            .map_err(|e| PaneError::Socket(format!("Failed to cast BPF map: {}", e)))?;

        // BPF C code reads IP addresses in network byte order (big endian)
        let ip_net_order = u32::from_ne_bytes(ip.octets());
        ip_groups
            .insert(ip_net_order, group_id, 0)
            .map_err(|e| PaneError::Socket(format!("Failed to insert IP into BPF map: {}", e)))?;
    }
    Ok(())
}

/// Unregisters a VM IPv4 address from the BPF map.
pub fn unregister_vm_network_group(ip: Ipv4Addr) -> Result<()> {
    init_network_ebpf()?;
    let mut guard = lock_bpf()?;
    if let Some(ref mut bpf) = *guard {
        // Retrieve ip_groups map
        let mut ip_groups: HashMap<_, u32, u32> =
            HashMap::try_from(bpf.map_mut("ip_groups").ok_or_else(|| {
                PaneError::Socket("ip_groups map not found in BPF object".to_string())
            })?)
            .map_err(|e| PaneError::Socket(format!("Failed to cast BPF map: {}", e)))?;

        let ip_net_order = u32::from_ne_bytes(ip.octets());
        let _ = ip_groups.remove(&ip_net_order);
    }
    Ok(())
}

/// Retrieves the registered micro-segmentation group ID for a VM IPv4 address.
pub fn get_vm_network_group(ip: Ipv4Addr) -> Result<Option<u32>> {
    init_network_ebpf()?;
    let mut guard = lock_bpf()?;
    if let Some(ref mut bpf) = *guard {
        let ip_groups: HashMap<_, u32, u32> =
            HashMap::try_from(bpf.map_mut("ip_groups").ok_or_else(|| {
                PaneError::Socket("ip_groups map not found in BPF object".to_string())
            })?)
            .map_err(|e| PaneError::Socket(format!("Failed to cast BPF map: {}", e)))?;

        let ip_net_order = u32::from_ne_bytes(ip.octets());
        match ip_groups.get(&ip_net_order, 0) {
            Ok(group_id) => return Ok(Some(group_id)),
            Err(_) => return Ok(None),
        }
    }
    Ok(None)
}
