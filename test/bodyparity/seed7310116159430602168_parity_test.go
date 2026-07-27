package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 7310116159430602168 (LevelC): residual free Acc-early between pure multi
// free-ref members of nested FE (func_26 multi free-ref g_165/g_348/g_205 with
// residual free g_276 Acc between g_348 and g_205). Place Acc-late pure multi
// free-ref mid before residual free of same FE (not free-ref on parent) when pure
// multi free-ref head Acc-early adjacent before residual free. Cascades to
// func_1/func_23. Session/FM-local — no package mutable state.
func TestSeed7310116159430602168(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 7310116159430602168
	assertOptsBodyParity(t, o)
}
