package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 4403953022868748150 (LevelC): (1) free-ref multi pure FE head g_13 of func_42
// Acc-late after residual free free-neither g_60 with free-ref multi mid g_46 Acc-
// early — Case A-res reorderMulti places g_46 g_13 g_60. (2) free residual pure free-
// ref free of free-head func_32 g_207 Acc-early between pure-head func_36 g_154 and
// free residual free of pure-head g_190; Acc g_190 g_101 g_207 — Acc invent Acc-order
// free residual pure free-ref free of free-head free-ref free of owner only Acc-early
// with gap when Acc puts free residual free currently after pure before pure; keep map
// for pure multi free-ref mid Acc pollution (seed7310). Session/FM-local — no package
// mutable state.
func TestSeed4403953022868748150(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 4403953022868748150
	assertOptsBodyParity(t, o)
}
