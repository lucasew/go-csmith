package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 639: pure mid IV g_295 of nested func_37 Acc residual late on parent
// func_31 (after g_287). pureOnlyRelativeOrder must not Acc-late-yank mid pure
// when free residual after pure free-refs only in nested Acc, not parent body.
func TestSeed639(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 639
	assertOptsBodyParity(t, o)
}
