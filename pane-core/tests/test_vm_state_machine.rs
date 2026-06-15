// SPDX-License-Identifier: Apache-2.0

use pane_core::backends::MachineConfig;
use pane_core::vm::{Dead, Frozen, Running, Spawning, Vm};

#[tokio::test]
async fn test_firecracker_spawning_to_dead() {
    // Check if firecracker is installed
    let fc_check = std::process::Command::new("firecracker")
        .arg("--version")
        .output();

    if fc_check.is_err() {
        println!("Skipping Firecracker state machine test: 'firecracker' binary not found.");
        return;
    }

    // 1. Initial State: Spawning
    let mut vm: Vm<Spawning> = Vm::new_firecracker("fc-state-test");
    assert_eq!(vm.id(), "fc-state-test");

    // Spawn underlying process
    vm.spawn().await.expect("Failed to spawn process");

    // Configure machine settings (only valid in Spawning state)
    let config = MachineConfig {
        vcpu_count: 1,
        mem_size_mib: 128,
        smt: Some(false),
        track_dirty_pages: Some(true),
    };
    vm.configure_machine(&config)
        .await
        .expect("Failed to configure machine");

    // 2. Transition: Spawning -> Dead
    let dead_vm: Vm<Dead> = vm.destroy().await.expect("Failed to destroy spawning VM");
    assert_eq!(dead_vm.id(), "fc-state-test");
}

#[tokio::test]
async fn test_native_full_lifecycle() {
    match pane_core::SafeVm::create() {
        Ok(safe_vm) => {
            // 1. Initial State: Spawning
            let vm: Vm<Spawning> = Vm::new_native("native-state-test", safe_vm);
            assert_eq!(vm.id(), "native-state-test");

            // 2. Transition: Spawning -> Running
            let running_vm: Vm<Running> =
                vm.start().await.expect("Failed to transition to Running");

            // 3. Transition: Running -> Frozen
            let frozen_vm: Vm<Frozen> = running_vm
                .freeze()
                .await
                .expect("Failed to transition to Frozen");

            // 4. Transition: Frozen -> Running
            let running_vm2: Vm<Running> = frozen_vm
                .resume()
                .await
                .expect("Failed to transition back to Running");

            // 5. Transition: Running -> Dead
            let dead_vm: Vm<Dead> = running_vm2
                .destroy()
                .await
                .expect("Failed to transition to Dead");
            assert_eq!(dead_vm.id(), "native-state-test");
        }
        Err(e) => {
            println!(
                "Skipping native state machine lifecycle test. KVM not accessible: {:?}",
                e
            );
        }
    }
}
