package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 48: free residual free of free-head Acc-early immediately before earliest
// own pure of parent on map while Acc has own pure before free residual free of
// free-head (g_105 free residual free of free-head func_66 FE Acc-early before
// own pure g_4 of func_1; Acc g_4 … g_105). Acc-order Acc-late. Array-init dim
// fors emit g_105 in C but IR for-IVs of free-head func_70 are params — not
// pure residual pure of free-head. Session/FM-local — no package mutable state.
func TestSeed48(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 48
	assertOptsBodyParity(t, o)
}
