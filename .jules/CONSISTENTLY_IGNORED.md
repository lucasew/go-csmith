## IGNORE: Superficial Documentation Comments

**- Pattern:** Adding obvious or superficial documentation comments solely to silence `golint` warnings on exported symbols.
**- Justification:** Documentation should explain the "why" and non-obvious nuances, not just restate the function name or its signature to pass automated checks.
**- Files Affected:** internal/cli/root.go, pkg/csmith/options.go, pkg/csmith/rng.go

## IGNORE: Insecure Temporary Files

**- Pattern:** Hardcoding `/tmp` paths and using world-readable permissions (e.g., `0644`) for trace or output files.
**- Justification:** Violates security conventions (CWE-377, CWE-276), allowing symlink attacks and unauthorized access. Use `os.TempDir()` or `os.CreateTemp()` and restrictive permissions like `0600`.
**- Files Affected:** pkg/csmith/rng.go

## IGNORE: Incorrect Error Handler Filename

**- Pattern:** Implementing the centralized error reporting function in files like `reporter.go` or `error.go`.
**- Justification:** The project mandates that the centralized error handler MUST be located specifically in `pkg/errorhandler/errorhandler.go`.
**- Files Affected:** pkg/errorhandler/reporter.go, pkg/errorhandler/error.go

## IGNORE: Unscoped Git Staging

**- Pattern:** Using `git add -A` or `git add .` to stage files inside CI workflows or scripts.
**- Justification:** Blanket staging violates scope discipline and risks accidentally committing unintended artifacts. Files must be staged explicitly by path.
**- Files Affected:** .github/workflows/autorelease.yml
