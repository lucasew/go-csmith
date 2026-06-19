## IGNORE: Superficial Documentation Comments

**- Pattern:** Adding obvious or superficial documentation comments solely to silence `golint` warnings on exported symbols.
**- Justification:** Documentation should explain the "why" and non-obvious nuances, not just restate the function name or its signature to pass automated checks.
**- Files Affected:** internal/cli/root.go, pkg/csmith/options.go, pkg/csmith/rng.go
