use pane_core::resources::{get_cgroup_base_path, CpuMaxLimit, ResourceControls};
use pane_core::vm::Vm;
use std::fs;

#[tokio::test]
async fn test_cgroups_lifecycle_and_limits() {
    // 1. Skip if firecracker is not installed
    let fc_check = std::process::Command::new("firecracker")
        .arg("--version")
        .output();

    if fc_check.is_err() {
        println!("Skipping cgroup test: 'firecracker' binary not found in PATH.");
        return;
    }

    // 2. Skip if cgroup base directory is not writeable (e.g., restricted containers)
    let base_path = match get_cgroup_base_path() {
        Ok(path) => path,
        Err(e) => {
            println!(
                "Skipping cgroup test: cgroup v2 base path not writeable: {:?}",
                e
            );
            return;
        }
    };

    let vm_id = "test-cgroup-vm";
    let cg_dir = base_path.join(vm_id);

    // Ensure any stale cgroups are cleaned up first
    if cg_dir.exists() {
        let _ = fs::remove_dir(&cg_dir);
    }

    // 3. Create a Spawning VM
    let mut vm = Vm::new_firecracker(vm_id);
    vm.spawn().await.expect("Failed to spawn firecracker VM");

    let pid = vm.pid().expect("VM should have a PID after spawn");

    // 4. Verify cgroup directory exists and contains the VM's PID
    assert!(cg_dir.exists(), "Cgroup directory should be created");

    let procs_content =
        fs::read_to_string(cg_dir.join("cgroup.procs")).expect("Failed to read cgroup.procs");

    let procs_pids: Vec<u32> = procs_content
        .lines()
        .filter_map(|line| line.parse::<u32>().ok())
        .collect();

    assert!(
        procs_pids.contains(&pid),
        "Cgroup procs {:?} should contain PID {}",
        procs_pids,
        pid
    );

    // 5. Apply resource controls
    let limits = ResourceControls {
        memory_max: Some(268435456),  // 256MB
        memory_high: Some(209715200), // 200MB
        cpu_weight: Some(150),
        cpu_max: Some(CpuMaxLimit {
            quota_us: 50000,
            period_us: 100000,
        }),
        pids_max: Some(20),
    };

    vm.apply_resources(&limits)
        .expect("Failed to apply resource controls");

    // 6. Verify limits are written correctly
    let mem_max_val: u64 = fs::read_to_string(cg_dir.join("memory.max"))
        .unwrap()
        .trim()
        .parse()
        .unwrap();
    assert_eq!(mem_max_val, 268435456);

    let mem_high_val: u64 = fs::read_to_string(cg_dir.join("memory.high"))
        .unwrap()
        .trim()
        .parse()
        .unwrap();
    assert_eq!(mem_high_val, 209715200);

    let cpu_weight_val: u32 = fs::read_to_string(cg_dir.join("cpu.weight"))
        .unwrap()
        .trim()
        .parse()
        .unwrap();
    assert_eq!(cpu_weight_val, 150);

    let cpu_max_str = fs::read_to_string(cg_dir.join("cpu.max")).unwrap();
    let cpu_max_parts: Vec<&str> = cpu_max_str.split_whitespace().collect();
    assert_eq!(cpu_max_parts[0], "50000");
    assert_eq!(cpu_max_parts[1], "100000");

    let pids_max_val: i64 = fs::read_to_string(cg_dir.join("pids.max"))
        .unwrap()
        .trim()
        .parse()
        .unwrap();
    assert_eq!(pids_max_val, 20);

    // 7. Destroy VM and verify cgroup directory is cleaned up
    let dead_vm = vm.destroy().await.expect("Failed to destroy VM");
    assert!(
        !cg_dir.exists(),
        "Cgroup directory should be cleaned up after VM destruction"
    );
    drop(dead_vm);
}
