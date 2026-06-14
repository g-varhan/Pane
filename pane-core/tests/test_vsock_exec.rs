use pane_core::exec::{exec_in_guest, ExecChunk, ExecRequest};
use std::path::Path;
use std::time::Instant;
use tokio::process::Command;

#[tokio::test]
async fn test_agent_exec_success() {
    let uds_path = Path::new("/tmp/pane-agent-test.sock");
    if uds_path.exists() {
        let _ = std::fs::remove_file(uds_path);
    }

    // Start guest agent in UDS mode
    let mut agent_proc = Command::new("./pane-agent")
        .arg("--uds")
        .arg(uds_path)
        .spawn()
        .expect("Failed to start pane-agent");

    // Give the agent socket a moment to initialize
    tokio::time::sleep(std::time::Duration::from_millis(100)).await;

    // Execute `/bin/echo` with arguments
    let req = ExecRequest {
        command: "/bin/echo".to_string(),
        args: vec!["hello".to_string(), "world".to_string()],
    };

    let mut stream = exec_in_guest(uds_path, &req, false)
        .await
        .expect("Failed to initiate exec");

    let mut stdout_bytes = Vec::new();
    let mut exit_code = None;

    while let Some(chunk) = stream.next().await.expect("Stream error") {
        match chunk {
            ExecChunk::Stdout(data) => stdout_bytes.extend_from_slice(&data),
            ExecChunk::Stderr(_) => panic!("Expected no stderr"),
            ExecChunk::ExitCode(code) => exit_code = Some(code),
        }
    }

    assert_eq!(String::from_utf8_lossy(&stdout_bytes), "hello world\n");
    assert_eq!(exit_code, Some(0));

    // Cleanup agent process
    let _ = agent_proc.kill().await;
    if uds_path.exists() {
        let _ = std::fs::remove_file(uds_path);
    }
}

#[tokio::test]
async fn test_agent_exec_stderr_and_failure() {
    let uds_path = Path::new("/tmp/pane-agent-test-err.sock");
    if uds_path.exists() {
        let _ = std::fs::remove_file(uds_path);
    }

    // Start guest agent in UDS mode
    let mut agent_proc = Command::new("./pane-agent")
        .arg("--uds")
        .arg(uds_path)
        .spawn()
        .expect("Failed to start pane-agent");

    // Give the agent socket a moment to initialize
    tokio::time::sleep(std::time::Duration::from_millis(100)).await;

    // Execute `/bin/sh` that exits with 42 and prints to stderr
    let req = ExecRequest {
        command: "/bin/sh".to_string(),
        args: vec![
            "-c".to_string(),
            "echo failure-message >&2; exit 42".to_string(),
        ],
    };

    let mut stream = exec_in_guest(uds_path, &req, false)
        .await
        .expect("Failed to initiate exec");

    let mut stderr_bytes = Vec::new();
    let mut exit_code = None;

    while let Some(chunk) = stream.next().await.expect("Stream error") {
        match chunk {
            ExecChunk::Stdout(_) => panic!("Expected no stdout"),
            ExecChunk::Stderr(data) => stderr_bytes.extend_from_slice(&data),
            ExecChunk::ExitCode(code) => exit_code = Some(code),
        }
    }

    assert_eq!(String::from_utf8_lossy(&stderr_bytes), "failure-message\n");
    assert_eq!(exit_code, Some(42));

    // Cleanup agent process
    let _ = agent_proc.kill().await;
    if uds_path.exists() {
        let _ = std::fs::remove_file(uds_path);
    }
}

#[tokio::test]
async fn test_agent_roundtrip_benchmark() {
    let uds_path = Path::new("/tmp/pane-agent-test-bench.sock");
    if uds_path.exists() {
        let _ = std::fs::remove_file(uds_path);
    }

    // Start guest agent in UDS mode
    let mut agent_proc = Command::new("./pane-agent")
        .arg("--uds")
        .arg(uds_path)
        .spawn()
        .expect("Failed to start pane-agent");

    tokio::time::sleep(std::time::Duration::from_millis(100)).await;

    let req = ExecRequest {
        command: "/bin/true".to_string(),
        args: vec![],
    };

    let start = Instant::now();
    let mut stream = exec_in_guest(uds_path, &req, false)
        .await
        .expect("Failed to initiate exec");

    let mut exit_code = None;
    while let Some(chunk) = stream.next().await.expect("Stream error") {
        if let ExecChunk::ExitCode(code) = chunk {
            exit_code = Some(code);
        }
    }
    let elapsed = start.elapsed();

    println!("Vsock exec round-trip took: {:?}", elapsed);
    assert_eq!(exit_code, Some(0));

    // Benchmark must be < 10ms on loopback
    assert!(
        elapsed < std::time::Duration::from_millis(10),
        "Round-trip execution took too long: {:?}",
        elapsed
    );

    let _ = agent_proc.kill().await;
    if uds_path.exists() {
        let _ = std::fs::remove_file(uds_path);
    }
}
