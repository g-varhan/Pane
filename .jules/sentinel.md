## 2024-05-18 - Host Environment Injection via Unsafe API Handler
**Vulnerability:** The Go API server `handler.go` took `spec.Env` (user-controlled environment variables meant for the spawned VM) and injected them directly into the host process using `os.Setenv` before making the FFI call.
**Learning:** `os.Setenv` modifies the global environment of the entire process. Doing this in a concurrent request handler is both a data race and a critical vulnerability (allowing an attacker to overwrite host configuration, `PATH`, or secrets).
**Prevention:** Never use `os.Setenv` to set request-specific state. Pass environment maps directly down through execution layers (FFI/exec wrappers) to ensure they only apply to the target child process or VM.
