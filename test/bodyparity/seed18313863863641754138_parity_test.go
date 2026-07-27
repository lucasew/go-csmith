package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 18313863863641754138 (LevelC): pureMiss-by-prev re-inverted visit order of
// pure-only pure FE head g_30 of soft-nested func_77 vs free-ref pure residual
// g_41 of intermediate func_17 (pairSwap FE residual consecutive matched prev).
// Keep visit for pure-only nested pure FE head + free-ref pure when inverted vs
// pure-only head. AccEarly before residual free must not jump past free-ref
// residual mid Acc (func_1 g_41; func_50 g_917.f5). Session/FM-local — no package
// mutable state.
func TestSeed18313863863641754138(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 18313863863641754138
	assertOptsBodyParity(t, o)
}
