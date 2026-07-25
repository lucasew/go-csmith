// Upstream: AbsProgramGenerator / DefaultProgramGenerator (goGenerator entry).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"context"
	"fmt"
)

// Generate emits a full C program (upstream stdout role).
// Pure entry: builds a fresh Session and runs s.Generate — no residual ambient
// session after return.
//
// Not safe for concurrent calls in one process (upstream is one generation per
// process). Fuzz workers are separate processes.
func Generate(opts Options) (string, error) {
	return NewSession(opts).Generate(context.Background())
}

// GenerateContext is the preferred package entry (SPEC §2.2).
func GenerateContext(ctx context.Context, opts Options) (string, error) {
	return NewSession(opts).Generate(ctx)
}

// Generate runs one generation using s as the only mutable bag for the run.
// Activates s for Process* helpers for the duration of the call, then clears
// the ambient pointer so nothing is left global.
func (s *Session) Generate(ctx context.Context) (string, error) {
	if s == nil {
		return "", fmt.Errorf("nil session")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	// Bridge: install this session for Process*/SetError for this call only.
	restore := activateSession(s)
	defer restore()

	opts := s.Opts
	// Platform resolve when sizes needed later
	if opts.PlatformInfoPath != "" {
		if resolved, err := opts.resolvePlatformInfo(); err == nil {
			opts = resolved
			s.Opts = opts
		}
	}
	// RandomProgramGenerator.cpp — fix_options_for_cpp / resolve_exhaustive
	opts = opts.normalizeUpstreamFlow()
	s.Opts = opts
	if err := opts.Validate(); err != nil {
		return "", err
	}
	ClearErrorSess(s)
	SetPlatformSizesSess(s, opts.IntSize, opts.PointerSize)
	if !InitPartialExpanderFromOptionsSess(s, opts) {
		return "", fmt.Errorf("invalid partial-expand: %q", opts.PartialExpand)
	}
	defer ClearPartialExpanderSess(s)

	g := NewProgramGenerator(s)
	if g == nil {
		return "", fmt.Errorf("nil program generator")
	}
	g.Argv = opts.Argv
	defer ClearAttrGeneratorsSess(s)

	if opts.DumpDefaultProbabilities != "" {
		if err := WriteDumpDefaultProbabilitiesSess(s, opts.DumpDefaultProbabilities); err != nil {
			return "", err
		}
		return "", nil
	}
	if opts.DumpRandomProbabilities != "" {
		if err := WriteDumpActualProbabilitiesSess(s, opts.DumpRandomProbabilities, opts.Seed); err != nil {
			return "", err
		}
		return "", nil
	}
	if opts.ProbabilityConfiguration != "" {
		p := s.Probs
		if p == nil {
			return "", fmt.Errorf("probabilities not initialized")
		}
		if msg, ok := p.ParseConfigurationSess(s, opts.ProbabilityConfiguration); !ok {
			return "", fmt.Errorf("parsing configuration file error: %s", msg)
		}
	}
	out := g.GoGenerator()
	if s.GenError != ErrSuccess {
		code := s.GenError
		s.GenError = ErrSuccess
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
