use crate::error::{PaneError, Result};
use serde::Serialize;
use tokio::io::{AsyncRead, AsyncReadExt, AsyncWriteExt};
use tokio::net::UnixStream;

/// A request to execute a command inside the VM.
///
/// # Example
/// ```
/// use pane_core::ExecRequest;
/// let req = ExecRequest {
///     command: "/bin/ls".to_string(),
///     args: vec!["-l".to_string()],
/// };
/// ```
#[derive(Serialize, Debug, Clone)]
pub struct ExecRequest {
    /// The binary or command to execute.
    pub command: String,
    /// Arguments to pass to the command.
    pub args: Vec<String>,
}

/// A chunk of output from the guest command execution.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ExecChunk {
    /// Data from stdout.
    Stdout(Vec<u8>),
    /// Data from stderr.
    Stderr(Vec<u8>),
    /// The final exit status code of the process.
    ExitCode(i32),
}

/// An asynchronous stream of execution chunks.
pub struct ExecStream<R> {
    reader: R,
    finished: bool,
}

impl<R: AsyncRead + Unpin> ExecStream<R> {
    /// Creates a new `ExecStream` from a reader.
    pub fn new(reader: R) -> Self {
        Self {
            reader,
            finished: false,
        }
    }

    /// Read the next chunk of output from the stream.
    ///
    /// Returns `None` when the stream is exhausted.
    ///
    /// # Example
    /// ```no_run
    /// # tokio_test::block_on(async {
    /// # use pane_core::ExecStream;
    /// # let reader = tokio::io::empty();
    /// let mut stream = ExecStream::new(reader);
    /// while let Some(chunk) = stream.next().await.unwrap() {
    ///     println!("{:?}", chunk);
    /// }
    /// # });
    /// ```
    pub async fn next(&mut self) -> Result<Option<ExecChunk>> {
        if self.finished {
            return Ok(None);
        }

        let mut header = [0u8; 5];
        match self.reader.read_exact(&mut header).await {
            Ok(_) => {
                let frame_type = header[0];
                let length =
                    u32::from_be_bytes([header[1], header[2], header[3], header[4]]) as usize;

                let mut payload = vec![0u8; length];
                self.reader.read_exact(&mut payload).await.map_err(|e| {
                    PaneError::Socket(format!(
                        "Failed to read frame payload of length {}: {}",
                        length, e
                    ))
                })?;

                match frame_type {
                    1 => Ok(Some(ExecChunk::Stdout(payload))),
                    2 => Ok(Some(ExecChunk::Stderr(payload))),
                    3 => {
                        self.finished = true;
                        if payload.len() != 4 {
                            return Err(PaneError::Socket(format!(
                                "Invalid exit code payload size: expected 4, got {}",
                                payload.len()
                            )));
                        }
                        let code =
                            i32::from_be_bytes([payload[0], payload[1], payload[2], payload[3]]);
                        Ok(Some(ExecChunk::ExitCode(code)))
                    }
                    t => Err(PaneError::Socket(format!("Unknown frame type: {}", t))),
                }
            }
            Err(ref e) if e.kind() == std::io::ErrorKind::UnexpectedEof => {
                self.finished = true;
                Ok(None)
            }
            Err(e) => Err(PaneError::from(e)),
        }
    }
}

/// Performs the Firecracker vsock UDS handshake on a given UnixStream.
pub async fn vsock_handshake(stream: &mut UnixStream, port: u32) -> Result<()> {
    let handshake = format!("CONNECT {}\n", port);
    stream
        .write_all(handshake.as_bytes())
        .await
        .map_err(|e| PaneError::Socket(format!("Failed to write vsock handshake: {}", e)))?;

    // Read the response until newline
    let mut response = Vec::new();
    let mut buf = [0u8; 1];
    loop {
        stream.read_exact(&mut buf).await.map_err(|e| {
            PaneError::Socket(format!("Failed to read vsock handshake response: {}", e))
        })?;
        response.push(buf[0]);
        if buf[0] == b'\n' {
            break;
        }
        if response.len() > 128 {
            return Err(PaneError::Socket("Handshake response too long".to_string()));
        }
    }

    let resp_str = String::from_utf8_lossy(&response);
    if !resp_str.starts_with("OK") {
        return Err(PaneError::Socket(format!(
            "Vsock handshake failed: {}",
            resp_str.trim()
        )));
    }

    Ok(())
}

/// Initiates connection to the guest, executes the command, and returns the stream of outputs.
pub async fn exec_in_guest(
    uds_path: &std::path::Path,
    req: &ExecRequest,
    use_handshake: bool,
) -> Result<ExecStream<UnixStream>> {
    let mut stream = UnixStream::connect(uds_path).await.map_err(|e| {
        PaneError::Socket(format!(
            "Failed to connect to guest vsock socket {:?}: {}",
            uds_path, e
        ))
    })?;

    if use_handshake {
        vsock_handshake(&mut stream, 1024).await?;
    }

    // Write JSON payload
    let payload = serde_json::to_string(req)?;
    stream
        .write_all(payload.as_bytes())
        .await
        .map_err(|e| PaneError::Socket(format!("Failed to write exec payload: {}", e)))?;

    // Shut down write half to signal EOF on JSON to the guest agent
    stream
        .shutdown()
        .await
        .map_err(|e| PaneError::Socket(format!("Failed to shut down stream write half: {}", e)))?;

    Ok(ExecStream::new(stream))
}
