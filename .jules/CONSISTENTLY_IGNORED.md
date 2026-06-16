## IGNORE: Superficial Documentation Comments

**- Pattern:** Adding obvious or superficial documentation comments solely to silence `golint` warnings on exported symbols.
**- Justification:** Documentation should explain the "why" and non-obvious nuances, not just restate the function name or its signature to pass automated checks.
**- Files Affected:** internal/cli/root.go, pkg/csmith/options.go

## IGNORE: Insecure Temporary Files

**- Pattern:** Hardcoding `/tmp` paths and using world-readable permissions (e.g., `0644`) for trace or output files.
**- Justification:** Violates security conventions (CWE-377, CWE-276), allowing symlink attacks and unauthorized access. Use `os.TempDir()` or `os.CreateTemp()` and restrictive permissions like `0600`.
**- Files Affected:** pkg/csmith/rng.go

## IGNORE: Premature Refactoring

**- Pattern:** Extracting code into new files (e.g., environment and state management) without citing academic principles (e.g., Fowler, GoF) or satisfying the Rule of Three for code duplication.
**- Justification:** Refactoring must be grounded in clear structural rationale and duplication patterns, not arbitrary file splitting.
**- Files Affected:** pkg/csmith/generator.go, pkg/csmith/env_gen.go, pkg/csmith/state.go

## IGNORE: Unscoped Git Staging

**- Pattern:** Using `git add -A` or `git add .` to stage files, such as inside GitHub Actions CI workflows.
**- Justification:** Blanket staging violates scope discipline and risks accidentally committing unintended artifacts. Files must be staged explicitly by path.
**- Files Affected:** .github/workflows/autorelease.yml
