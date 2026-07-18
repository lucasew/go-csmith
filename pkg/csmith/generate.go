// Upstream: AbsProgramGenerator / DefaultProgramGenerator (goGenerator entry).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"context"
	"fmt"
)

// Generate emits a full C program (upstream stdout role).
// Wires DefaultProgramGenerator initialize → goGenerator.
func Generate(opts Options) (string, error) {
	return GenerateContext(context.Background(), opts)
}

// GenerateContext is the preferred entry (SPEC §2.2).
func GenerateContext(ctx context.Context, opts Options) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	// Platform resolve when sizes needed later
	if opts.PlatformInfoPath != "" {
		if resolved, err := opts.resolvePlatformInfo(); err == nil {
			opts = resolved
		}
	}
	// Type::SizeInBytes uses platform integer/pointer sizes
	SetPlatformSizes(opts.IntSize, opts.PointerSize)
	// PartialExpander from CGOptions::partial_expand
	if !InitPartialExpanderFromOptions(opts) {
		return "", fmt.Errorf("invalid partial-expand: %q", opts.PartialExpand)
	}
	defer ClearPartialExpander()
	g := NewProgramGenerator(opts)
	out := g.GoGenerator()
	if out == "" {
		return "", fmt.Errorf("empty program generation")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return out, nil
}
