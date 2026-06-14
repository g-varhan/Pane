use std::path::{Path, PathBuf};
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::UnixStream;
use serde::{Serialize, Deserialize};
use crate::error::{PaneError, Result};

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct MachineConfig {
    pub vcpu_count: u32,
    pub mem_size_mib: u32,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub smt: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub track_dirty_pages: Option<bool>,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct BootSource {
    pub kernel_image_path: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub boot_args: Option<String>,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct Drive {
    pub drive_id: String,
    pub path_on_host: String,
    pub is_root_device: bool,
    pub is_read_only: bool,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
struct Action {
    action_type: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
struct SnapshotCreate {
    snapshot_path: String,
    mem_file_path: String,
    snapshot_type: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
struct SnapshotLoad {
    snapshot_path: String,
    mem_file_path: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    resume_vm: Option<bool>,
}

pub struct FirecrackerVm {
    id: String,
    socket_path: PathBuf,
    child: Option<tokio::process::Child>,
}

impl FirecrackerVm {
    /// Initialize a new Firecracker VM instance.
    pub fn new(id: &str) -> Self {
        let socket_path = Self::get_socket_path(id);
        Self {
            id: id.to_string(),
            socket_path,
            child: None,
        }
    }

    /// Returns the ID of the VM.
    pub fn id(&self) -> &str {
        &self.id
    }

    /// Helper to resolve the socket path, falling back to /tmp/pane if /run/pane is not writable.
    fn get_socket_path(vm_id: &str) -> PathBuf {
        let run_pane = Path::new("/run/pane");
        if run_pane.exists() || std::fs::create_dir_all(run_pane).is_ok() {
            run_pane.join(format!("fc-{}.sock", vm_id))
        } else {
            let tmp_pane = Path::new("/tmp/pane");
            let _ = std::fs::create_dir_all(tmp_pane);
            tmp_pane.join(format!("fc-{}.sock", vm_id))
        }
    }

    /// Returns the socket path of the VM.
    pub fn socket_path(&self) -> &Path {
        &self.socket_path
    }

    /// Spawns the firecracker process and waits for the Unix socket to be ready.
    pub async fn spawn(&mut self) -> Result<()> {
        if self.child.is_some() {
            return Err(PaneError::Spawn("Firecracker VM already spawned".to_string()));
        }

        // Ensure stale socket files are cleaned up
        if self.socket_path.exists() {
            let _ = std::fs::remove_file(&self.socket_path);
        }

        let log_path = self.socket_path.with_extension("log");
        let log_file = std::fs::File::create(&log_path).map_err(|e| {
            PaneError::Spawn(format!("Failed to create log file {:?}: {}", log_path, e))
        })?;

        let mut cmd = tokio::process::Command::new("firecracker");
        cmd.arg("--api-sock").arg(&self.socket_path);
        cmd.stdout(log_file.try_clone().unwrap());
        cmd.stderr(log_file);

        let child = cmd.spawn().map_err(|e| {
            PaneError::Spawn(format!("Failed to execute firecracker binary: {}", e))
        })?;
        self.child = Some(child);

        // Wait for the socket to accept connections
        let start = std::time::Instant::now();
        let timeout = std::time::Duration::from_secs(5);
        let mut connected = false;

        while start.elapsed() < timeout {
            // Check if process has terminated prematurely
            if let Some(ref mut c) = self.child {
                if let Ok(Some(status)) = c.try_wait() {
                    return Err(PaneError::Spawn(format!(
                        "Firecracker process terminated immediately with exit status: {}",
                        status
                    )));
                }
            }

            if UnixStream::connect(&self.socket_path).await.is_ok() {
                connected = true;
                break;
            }
            tokio::time::sleep(std::time::Duration::from_millis(50)).await;
        }

        if !connected {
            return Err(PaneError::Timeout(format!(
                "Waiting for Firecracker API socket {:?}",
                self.socket_path
            )));
        }

        Ok(())
    }

    /// Sends a lightweight HTTP request over the Unix socket.
    async fn send_request(&self, method: &str, path: &str, body: Option<&str>) -> Result<(String, String)> {
        let mut stream = UnixStream::connect(&self.socket_path).await.map_err(|e| {
            PaneError::Socket(format!("Failed to connect to socket {:?}: {}", self.socket_path, e))
        })?;

        let mut req = format!(
            "{} {} HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n",
            method, path
        );
        if let Some(b) = body {
            req.push_str(&format!(
                "Content-Type: application/json\r\nContent-Length: {}\r\n",
                b.len()
            ));
        }
        req.push_str("\r\n");
        if let Some(b) = body {
            req.push_str(b);
        }

        stream.write_all(req.as_bytes()).await?;
        stream.flush().await?;

        let mut response_bytes = Vec::new();
        stream.read_to_end(&mut response_bytes).await?;

        let response_str = String::from_utf8_lossy(&response_bytes).into_owned();

        let mut parts = response_str.splitn(2, "\r\n\r\n");
        let headers_part = parts.next().unwrap_or("");
        let body_part = parts.next().unwrap_or("").to_string();

        let mut lines = headers_part.lines();
        let status_line = lines.next().ok_or_else(|| {
            PaneError::Socket("Malformed HTTP response: empty headers".to_string())
        })?;

        let status_parts: Vec<&str> = status_line.splitn(3, ' ').collect();
        if status_parts.len() < 2 {
            return Err(PaneError::Socket(format!(
                "Malformed HTTP status line: {}",
                status_line
            )));
        }
        let status_code = status_parts[1];

        if !status_code.starts_with('2') {
            return Err(PaneError::Api {
                status: status_code.to_string(),
                body: body_part,
            });
        }

        Ok((status_code.to_string(), body_part))
    }

    /// Configures the VM CPU and memory.
    pub async fn configure_machine(&self, config: &MachineConfig) -> Result<()> {
        let body = serde_json::to_string(config)?;
        self.send_request("PUT", "/machine-config", Some(&body)).await?;
        Ok(())
    }

    /// Configures the kernel boot source.
    pub async fn configure_boot_source(&self, boot: &BootSource) -> Result<()> {
        let body = serde_json::to_string(boot)?;
        self.send_request("PUT", "/boot-source", Some(&body)).await?;
        Ok(())
    }

    /// Configures a host file backing a VM drive.
    pub async fn configure_drive(&self, drive: &Drive) -> Result<()> {
        let body = serde_json::to_string(drive)?;
        let path = format!("/drives/{}", drive.drive_id);
        self.send_request("PUT", &path, Some(&body)).await?;
        Ok(())
    }

    /// Starts the VM instance.
    pub async fn start(&self) -> Result<()> {
        let body = serde_json::to_string(&Action {
            action_type: "InstanceStart".to_string(),
        })?;
        self.send_request("PUT", "/actions", Some(&body)).await?;
        Ok(())
    }

    /// Pauses the VM instance.
    pub async fn pause(&self) -> Result<()> {
        let body = serde_json::to_string(&Action {
            action_type: "Pause".to_string(),
        })?;
        self.send_request("PUT", "/actions", Some(&body)).await?;
        Ok(())
    }

    /// Resumes the VM instance from paused state.
    pub async fn resume(&self) -> Result<()> {
        let body = serde_json::to_string(&Action {
            action_type: "Resume".to_string(),
        })?;
        self.send_request("PUT", "/actions", Some(&body)).await?;
        Ok(())
    }

    /// Pauses, takes a snapshot of the VM's state, and resumes it.
    pub async fn create_snapshot(&self, snapshot_path: &str, mem_file_path: &str) -> Result<()> {
        self.pause().await?;
        let payload = SnapshotCreate {
            snapshot_path: snapshot_path.to_string(),
            mem_file_path: mem_file_path.to_string(),
            snapshot_type: "Full".to_string(),
        };
        let body = serde_json::to_string(&payload)?;
        let res = self.send_request("PUT", "/snapshot/create", Some(&body)).await;
        // Always attempt to resume even if snapshot creation fails
        let _ = self.resume().await;
        res?;
        Ok(())
    }

    /// Loads a snapshot and resumes execution.
    pub async fn load_snapshot(&self, snapshot_path: &str, mem_file_path: &str) -> Result<()> {
        let payload = SnapshotLoad {
            snapshot_path: snapshot_path.to_string(),
            mem_file_path: mem_file_path.to_string(),
            resume_vm: Some(true),
        };
        let body = serde_json::to_string(&payload)?;
        self.send_request("PUT", "/snapshot/load", Some(&body)).await?;
        Ok(())
    }

    /// Standard kill implementation.
    pub async fn kill(&mut self) -> Result<()> {
        if let Some(ref mut child) = self.child {
            let _ = child.kill().await;
            self.child = None;
        }
        if self.socket_path.exists() {
            let _ = std::fs::remove_file(&self.socket_path);
        }
        Ok(())
    }
}

impl Drop for FirecrackerVm {
    fn drop(&mut self) {
        if let Some(ref mut child) = self.child {
            let _ = child.start_kill();
        }
        if self.socket_path.exists() {
            let _ = std::fs::remove_file(&self.socket_path);
        }
    }
}
