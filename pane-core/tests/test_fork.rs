// SPDX-License-Identifier: Apache-2.0

use pane_core::vm::Vm;
use std::time::Instant;
use tokio::process::Command;

#[tokio::test]
async fn test_fork_50_vms() {
    let fc_check = Command::new("firecracker").arg("--version").output().await;
    if fc_check.is_err() {
        println!("Skipping benchmark: 'firecracker' binary not found.");
        return;
    }

    let snapshot_path = "/tmp/dummy_snapshot.snap";
    let mem_file_path = "/tmp/dummy_mem.file";

    let _ = std::fs::write(snapshot_path, b"dummy snapshot");
    let _ = std::fs::write(mem_file_path, b"dummy mem file");

    let start = Instant::now();

    let mut tasks = Vec::new();
    for i in 0..50 {
        let id = format!("fc-fork-bench-{}", i);
        let snap = snapshot_path.to_string();
        let mem = mem_file_path.to_string();

        tasks.push(tokio::spawn(async move {
            let res = Vm::fork_firecracker(&id, &snap, &mem).await;
            if let Ok(mut frozen_vm) = res {
                let _ = frozen_vm
                    .patch_drive("rootfs", &format!("/tmp/rootfs-{}.img", i))
                    .await;
                let _ = frozen_vm.configure_vsock(3 + i as u32).await;
                if let Ok(running_vm) = frozen_vm.resume().await {
                    let _ = running_vm.destroy().await;
                }
            }
        }));
    }

    for task in tasks {
        let _ = task.await;
    }

    let elapsed = start.elapsed();
    println!("Forked 50 VMs in {:?}", elapsed);
    assert!(
        elapsed < std::time::Duration::from_secs(2),
        "Forking 50 VMs took too long: {:?}",
        elapsed
    );

    let _ = std::fs::remove_file(snapshot_path);
    let _ = std::fs::remove_file(mem_file_path);
}

#[test]
fn test_cow_clone_fast_fail() {
    let src = "/tmp/test_cow_clone_src";
    let dst = "/tmp/test_cow_clone_dst";

    let _ = std::fs::write(src, b"source data");
    let _ = std::fs::remove_file(dst);

    let res = pane_core::vm::cow_clone_rootfs(src, dst);
    assert!(res.is_err());
    if let Err(pane_core::error::PaneError::Io(e)) = res {
        assert_eq!(e.raw_os_error(), Some(libc::ENOTSUP));
    } else {
        panic!("Expected PaneError::Io(ENOTSUP)");
    }

    let _ = std::fs::remove_file(src);
    let _ = std::fs::remove_file(dst);
}
