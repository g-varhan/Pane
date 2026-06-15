// SPDX-License-Identifier: Apache-2.0

use pane_core::backends::{BootSource, FirecrackerVm, MachineConfig};

#[tokio::test]
async fn test_firecracker_spawn_and_configure() {
    // Check if firecracker binary is installed
    let fc_check = std::process::Command::new("firecracker")
        .arg("--version")
        .output();

    if fc_check.is_err() {
        println!("Skipping Firecracker API test: 'firecracker' binary not found in PATH.");
        return;
    }

    let mut vm = FirecrackerVm::new("test-api-vm");

    // Spawn firecracker
    vm.spawn().await.expect("Failed to spawn Firecracker");

    // Set machine configuration
    let config = MachineConfig {
        vcpu_count: 1,
        mem_size_mib: 128,
        smt: Some(false),
        track_dirty_pages: Some(false),
    };

    vm.configure_machine(&config)
        .await
        .expect("Failed to configure machine");

    // Try to configure a non-existent boot source - should fail with an API error,
    // which confirms the API is receiving and parsing our request!
    let boot = BootSource {
        kernel_image_path: "/nonexistent/kernel".to_string(),
        boot_args: None,
    };

    let err = vm.configure_boot_source(&boot).await;
    assert!(
        err.is_err(),
        "Expected boot source configuration with nonexistent path to fail"
    );

    match err {
        Err(pane_core::PaneError::Api { status, body }) => {
            println!("Got expected API error (status {}): {}", status, body);
            assert_eq!(status, "400");
        }
        other => panic!("Expected API error, got: {:?}", other),
    }

    // Kill VM
    vm.kill().await.expect("Failed to kill Firecracker VM");
}
