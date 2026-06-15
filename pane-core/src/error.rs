// SPDX-License-Identifier: Apache-2.0

use thiserror::Error;

#[derive(Error, Debug)]
pub enum PaneError {
    #[error("VMM native error (errno {0}): {1}")]
    Vmm(i32, String),

    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),

    #[error("JSON error: {0}")]
    Json(#[from] serde_json::Error),

    #[error("Unix socket error: {0}")]
    Socket(String),

    #[error("Failed to spawn Firecracker process: {0}")]
    Spawn(String),

    #[error("Firecracker API returned error status {status}: {body}")]
    Api { status: String, body: String },

    #[error("Timeout waiting for {0}")]
    Timeout(String),
}

pub type Result<T> = std::result::Result<T, PaneError>;

/// Helper to check an FFI return value (negative errno on failure) and convert it to a Result.
pub fn check_ffi(code: i32, context: &str) -> Result<i32> {
    if code < 0 {
        let errno = -code;
        let msg = std::io::Error::from_raw_os_error(errno).to_string();
        Err(PaneError::Vmm(errno, format!("{}: {}", context, msg)))
    } else {
        Ok(code)
    }
}
