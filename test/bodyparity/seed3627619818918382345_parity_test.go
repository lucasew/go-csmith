package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 3627619818918382345 (LevelC): deeper pure FE head g_109 of func_57 Acc-late
// on func_1. pureOnly relative placeFE must prefer intermediate direct callee
// func_30 that transitively contains the pure-IV owner (g_113 g_109 g_114), not
// residual-only sibling func_18 (g_168 g_109). Session/FM-local — no package
// mutable state.
func TestSeed3627619818918382345(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 3627619818918382345
	assertOptsBodyParity(t, o)
}
