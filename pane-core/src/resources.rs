use crate::error::{PaneError, Result};
use std::fs;
use std::path::{Path, PathBuf};
use std::time::Duration;

/// Represents CPU quota constraints for a VM's cgroup.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct CpuMaxLimit {
    /// CPU execution quota in microseconds per period.
    pub quota_us: u64,
    /// CPU scheduling period in microseconds (usually 100,000us = 100ms).
    pub period_us: u64,
}

/// Represents resource limit configurations for a VM's cgroup v2 controls.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct ResourceControls {
    /// Hard memory limit in bytes (`memory.max`). None means unlimited.
    pub memory_max: Option<u64>,
    /// Soft memory limit in bytes (`memory.high`). None means unlimited.
    pub memory_high: Option<u64>,
    /// CPU scheduling weight (`cpu.weight`), 1 to 10000.
    pub cpu_weight: Option<u32>,
    /// CPU max quota limits (`cpu.max`).
    pub cpu_max: Option<CpuMaxLimit>,
    /// Maximum number of processes/tasks inside the cgroup (`pids.max`).
    /// Negative value means unlimited ("max").
    pub pids_max: Option<i64>,
}

/// Helper to discover the writeable base cgroup directory for Pane.
pub fn get_cgroup_base_path() -> Result<PathBuf> {
    // 1. Try standard root/system-wide path /sys/fs/cgroup/pane
    let root_path = PathBuf::from("/sys/fs/cgroup/pane");
    if root_path.exists() || fs::create_dir_all(&root_path).is_ok() {
        return Ok(root_path);
    }

    // 2. Try user-specific systemd cgroup path for non-root execution
    let uid = unsafe { libc::getuid() };
    if uid != 0 {
        let user_path = PathBuf::from(format!(
            "/sys/fs/cgroup/user.slice/user-{}.slice/user@{}.service/pane",
            uid, uid
        ));
        if user_path.exists() || fs::create_dir_all(&user_path).is_ok() {
            // Attempt to enable controllers in the parent user service path
            if let Some(parent) = user_path.parent() {
                let subtree_file = parent.join("cgroup.subtree_control");
                if subtree_file.exists() {
                    let _ = fs::write(&subtree_file, "+cpu +memory +pids");
                }
            }
            // Enable controllers in the pane base directory too
            let subtree_file = user_path.join("cgroup.subtree_control");
            let _ = fs::write(&subtree_file, "+cpu +memory +pids");
            return Ok(user_path);
        }
    }

    // 3. Fallback to reading /proc/self/cgroup and climbing up to find a writeable cgroup directory
    if let Ok(content) = fs::read_to_string("/proc/self/cgroup") {
        for line in content.lines() {
            let parts: Vec<&str> = line.split(':').collect();
            if parts.len() >= 3 && (parts[1].is_empty() || parts[1] == "name=systemd") {
                let rel_path = parts[2].trim_start_matches('/');
                let mut path = PathBuf::from("/sys/fs/cgroup").join(rel_path);
                while path.as_os_str() != "/sys/fs/cgroup" && path.as_os_str() != "/" {
                    if fs::metadata(&path)
                        .map(|m| !m.permissions().readonly())
                        .unwrap_or(false)
                    {
                        let pane_path = path.join("pane");
                        if pane_path.exists() || fs::create_dir_all(&pane_path).is_ok() {
                            let subtree_file = pane_path.join("cgroup.subtree_control");
                            let _ = fs::write(&subtree_file, "+cpu +memory +pids");
                            return Ok(pane_path);
                        }
                    }
                    if let Some(parent) = path.parent() {
                        path = parent.to_path_buf();
                    } else {
                        break;
                    }
                }
            }
        }
    }

    Err(PaneError::Io(std::io::Error::new(
        std::io::ErrorKind::PermissionDenied,
        "No writeable cgroup v2 directory found",
    )))
}

/// Manages a dedicated cgroup v2 directory for a Virtual Machine.
#[derive(Debug)]
pub struct CgroupManager {
    path: PathBuf,
}

impl CgroupManager {
    /// Creates and configures a new cgroup v2 for a VM.
    pub fn create(vm_id: &str) -> Result<Self> {
        let base = get_cgroup_base_path()?;
        let path = base.join(vm_id);

        if !path.exists() {
            fs::create_dir_all(&path).map_err(|e| {
                PaneError::Io(std::io::Error::new(
                    e.kind(),
                    format!("Failed to create cgroup directory for VM {}: {}", vm_id, e),
                ))
            })?;
        }

        Ok(Self { path })
    }

    /// Returns the absolute path of this VM's cgroup.
    pub fn path(&self) -> &Path {
        &self.path
    }

    /// Attaches a process PID to this cgroup.
    pub fn attach_pid(&self, pid: u32) -> Result<()> {
        let procs_file = self.path.join("cgroup.procs");
        fs::write(&procs_file, pid.to_string()).map_err(|e| {
            PaneError::Io(std::io::Error::new(
                e.kind(),
                format!(
                    "Failed to write PID {} to {}: {}",
                    pid,
                    procs_file.display(),
                    e
                ),
            ))
        })?;
        Ok(())
    }

    /// Applies the resource controls to the cgroup limits.
    pub fn apply(&self, limits: &ResourceControls) -> Result<()> {
        // 1. Memory limits
        if let Some(mem_max) = limits.memory_max {
            fs::write(self.path.join("memory.max"), mem_max.to_string())?;
        }
        if let Some(mem_high) = limits.memory_high {
            fs::write(self.path.join("memory.high"), mem_high.to_string())?;
        }

        // 2. CPU scheduling limits
        if let Some(weight) = limits.cpu_weight {
            fs::write(self.path.join("cpu.weight"), weight.to_string())?;
        }
        if let Some(cpu_max) = limits.cpu_max {
            fs::write(
                self.path.join("cpu.max"),
                format!("{} {}", cpu_max.quota_us, cpu_max.period_us),
            )?;
        }

        // 3. PIDs limits
        if let Some(pids_limit) = limits.pids_max {
            let val = if pids_limit < 0 {
                "max".to_string()
            } else {
                pids_limit.to_string()
            };
            fs::write(self.path.join("pids.max"), val)?;
        }

        Ok(())
    }

    /// Destroys this cgroup and cleans up its directory.
    pub fn destroy(self) -> Result<()> {
        let kill_file = self.path.join("cgroup.kill");
        if kill_file.exists() {
            let _ = fs::write(&kill_file, "1");
        }

        // It can take a moment for the kernel to release all tasks/resources.
        // We will retry deletion with small backoffs.
        let mut retries = 5;
        while retries > 0 {
            if fs::remove_dir(&self.path).is_ok() {
                return Ok(());
            }
            std::thread::sleep(Duration::from_millis(50));
            retries -= 1;
        }

        // Final attempt, propagate error if it fails
        fs::remove_dir(&self.path).map_err(|e| {
            PaneError::Io(std::io::Error::new(
                e.kind(),
                format!(
                    "Failed to remove cgroup directory {}: {}",
                    self.path.display(),
                    e
                ),
            ))
        })?;

        Ok(())
    }
}
