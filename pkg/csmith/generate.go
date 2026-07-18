// Upstream: AbsProgramGenerator / DefaultProgramGenerator (not yet ported).
// Fair rewrite: Generate stays unimplemented until bottom-up spine exists (SPEC §2, §5).
package csmith

import (
	"context"
	"errors"
	"fmt"
)

// ErrNotImplemented is returned until DefaultProgramGenerator is ported fairly.
var ErrNotImplemented = errors.New("program generation not implemented (fair rewrite in progress)")

// Generate will emit a full C program (upstream goGenerator / stdout role).
// Until the C++-linked spine lands, this returns ErrNotImplemented.
func Generate(opts Options) (string, error) {
	return GenerateContext(context.Background(), opts)
}

// GenerateContext is the preferred entry (SPEC §2.2); context honored at coarse boundaries later.
func GenerateContext(ctx context.Context, opts Options) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("%w: port DefaultProgramGenerator bottom-up per SPEC", ErrNotImplemented)
}
