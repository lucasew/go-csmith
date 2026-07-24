// Upstream: AbsProgramGenerator / DefaultProgramGenerator (goGenerator entry).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"context"
	"fmt"
)

// Generate emits a full C program (upstream stdout role).
// Wires DefaultProgramGenerator initialize → goGenerator.
//
// Not safe for concurrent calls in one process (upstream is one generation per
// process). Fuzz workers are separate processes. Mutable state is session-
// specific (see session.go); no generateMu.
func Generate(opts Options) (string, error) {
	return GenerateContext(context.Background(), opts)
}

// GenerateContext is the preferred entry (SPEC §2.2).
// ctx cancel/deadline is checked at coarse boundaries (entry / after emit).
func GenerateContext(ctx context.Context, opts Options) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	// Fresh session for this run (unit-test defaultSession stays out of the way).
	restore := BeginGenerateSession()
	defer restore()

	// Platform resolve when sizes needed later
	if opts.PlatformInfoPath != "" {
		if resolved, err := opts.resolvePlatformInfo(); err == nil {
			opts = resolved
		}
	}
	// RandomProgramGenerator.cpp — fix_options_for_cpp / resolve_exhaustive side effects
	// then CGOptions::has_conflict() before generation.
	opts = opts.normalizeUpstreamFlow()
	if err := opts.Validate(); err != nil {
		return "", err
	}
	// Error::set_error(SUCCESS) for a clean generation run
	ClearError()
	// Type::SizeInBytes uses platform integer/pointer sizes
	SetPlatformSizes(opts.IntSize, opts.PointerSize)
	// PartialExpander from CGOptions::partial_expand
	if !InitPartialExpanderFromOptions(opts) {
		return "", fmt.Errorf("invalid partial-expand: %q", opts.PartialExpand)
	}
	defer ClearPartialExpander()
	// Session Probabilities + attr gens: NewProgramGenerator (C++ singleton); no invent
	// a second NewProbabilities(opts) here for InitAttrGenerators alone.
	g := NewProgramGenerator(opts)
	g.Argv = opts.Argv
	defer ClearAttrGenerators()
	// AbsProgramGenerator.cpp:64–86 — dump/parse probabilities then exit or continue
	if opts.DumpDefaultProbabilities != "" {
		if err := WriteDumpDefaultProbabilities(opts.DumpDefaultProbabilities); err != nil {
			return "", err
		}
		return "", nil // upstream exits after dump
	}
	if opts.DumpRandomProbabilities != "" {
		if err := WriteDumpActualProbabilities(opts.DumpRandomProbabilities, opts.Seed); err != nil {
			return "", err
		}
		return "", nil
	}
	if opts.ProbabilityConfiguration != "" {
		p := ProcessProbabilities()
		if p == nil {
			return "", fmt.Errorf("probabilities not initialized")
		}
		if msg, ok := p.ParseConfiguration(opts.ProbabilityConfiguration); !ok {
			return "", fmt.Errorf("parsing configuration file error: %s", msg)
		}
	}
	out := g.GoGenerator()
	// sticky ERROR_RETURN / failed make_first → empty out (no soft invent success)
	if HasError() {
		code := GetError()
		ClearError()
		return "", fmt.Errorf("generation error (Error=%d)", code)
	}
	if out == "" {
		return "", fmt.Errorf("empty program generation")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return out, nil
}
