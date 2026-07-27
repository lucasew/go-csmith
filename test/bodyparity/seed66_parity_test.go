package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 66: free residual pure free-ref free of free-head Acc-late on parent when
// free-ref free is only on a direct free-head free residual pure free-ref free
// owner (func_39 g_652 / g_292.f0 of free-head func_41 free-ref free of owner
// only — Acc before g_385; map late after g_283; UP after g_338 before g_385).
// Deeper-only free-head free residual pure free-ref free owners keep map
// (seed48 g_129.f2). Session/FM-local — no package mutable state.
func TestSeed66(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 66
	assertOptsBodyParity(t, o)
}
