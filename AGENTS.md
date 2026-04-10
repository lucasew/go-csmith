# Project Conventions & Operational Memory

## Directory Layout
- `cmd/**` -> Entrypoints.
- `pkg/**` -> Reusable core packages.
- `internal/**` -> Internal packages.
- `tests/**` -> Tests.
- `.jules/**` -> Agent scratchpad and configuration.

## Error Handling
- **Centralized Error Reporting:** All unexpected errors MUST be routed through `pkg/errorhandler.ReportError`. Direct logging (like `fmt.Fprintln` to stderr) for unexpected errors at the call site or empty catch blocks are strictly forbidden.

## Tooling
- Use `mise` for environment setup.
