// SPDX-License-Identifier: Apache-2.0

use pane_core::network::{
    get_vm_network_group, init_network_ebpf, register_vm_network_group, unregister_vm_network_group,
};
use std::net::Ipv4Addr;

#[test]
fn test_network_ebpf_groups() {
    // 1. Initialize eBPF loading
    if let Err(e) = init_network_ebpf() {
        println!(
            "Skipping network test: eBPF not supported in this environment: {:?}",
            e
        );
        return;
    }

    let ip_a = Ipv4Addr::new(10, 0, 0, 2);
    let ip_b = Ipv4Addr::new(10, 0, 0, 3);
    let ip_c = Ipv4Addr::new(10, 0, 0, 4);

    // Ensure state is clean before we start
    let _ = unregister_vm_network_group(ip_a);
    let _ = unregister_vm_network_group(ip_b);
    let _ = unregister_vm_network_group(ip_c);

    // 2. Register IPs in network groups
    register_vm_network_group(ip_a, 100).expect("Failed to register ip_a");
    register_vm_network_group(ip_b, 100).expect("Failed to register ip_b");
    register_vm_network_group(ip_c, 200).expect("Failed to register ip_c");

    // 3. Query group memberships from eBPF map
    let group_a = get_vm_network_group(ip_a).expect("Failed to query group_a");
    let group_b = get_vm_network_group(ip_b).expect("Failed to query group_b");
    let group_c = get_vm_network_group(ip_c).expect("Failed to query group_c");

    assert_eq!(group_a, Some(100));
    assert_eq!(group_b, Some(100));
    assert_eq!(group_c, Some(200));

    // 4. Unregister ip_a
    unregister_vm_network_group(ip_a).expect("Failed to unregister ip_a");
    let group_a_deleted = get_vm_network_group(ip_a).expect("Failed to query deleted group_a");
    assert_eq!(group_a_deleted, None);

    // Cleanup rest
    let _ = unregister_vm_network_group(ip_b);
    let _ = unregister_vm_network_group(ip_c);
}
