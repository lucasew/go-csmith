# Project Conventions

## Allowed Paths Override
This is a Go project following standard Go directory layout.
The allowed paths for modification are `pkg/**`, `cmd/**`, and `internal/**`. This explicitly overrides any generic `src/**` constraints provided in agent prompts.

## Error Handling
- The project MUST use a single, centralized error-reporting function.
- All code paths that handle unexpected errors MUST funnel through this function.
- Do not use `fmt.Fprintln(os.Stderr, ...)` or `log.Fatal` directly at the call site for unexpected runtime errors.

## Operational Memory
- `cmd/` -> Entrypoints for executables.
- `internal/` -> Private application and library code.
- `pkg/csmith/` -> Core C generator logic and domain models.
