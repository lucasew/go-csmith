# Project Guidelines

- Follow a standard Go directory layout. Allowed paths for modifications are `pkg/**`, `cmd/**`, and `internal/**`.
- Use exact versions for tool pinning in `mise.toml` (e.g., `go = "1.26.0"`).
- All unexpected errors MUST be routed through a single, centralized error-reporting function. Direct logging (like `fmt.Fprintln` to stderr) for unexpected errors at the call site or empty catch blocks are strictly forbidden.
- Use standard Go tools for build, test, format, and lint: `go build ./...`, `go test ./...`, `go fmt ./...`, `staticcheck ./...`, and `golint ./...`.
- Avoid overly aggressive dead-code deletion as they may be intentional stubs.
