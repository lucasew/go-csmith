package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Regression: ItemizeArray offset used MakeInt (mark_mutable wrap) while C++ uses
// int2str — seed 145079 + --mark-mutable-const diverged as (p_2 + (1)) vs (p_2 + 1).
func TestSeed145079MarkMutable(t *testing.T) {
	opts := csmith.Defaults()
	opts.Seed = 145079
	opts.MarkMutableConst = true
	assertOptsBodyParity(t, opts)
}
