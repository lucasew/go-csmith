package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 7882445078581404101 (LevelC): pure-prefix-before-free must not yank nested
// pure FE head g_63 of func_76 before free residual that is own pure of parent
// (g_46 of func_1). seed88 free residual g_91 is not own pure of parent.
// Session/FM-local — no package mutable state.
func TestSeed7882445078581404101(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 7882445078581404101
	assertOptsBodyParity(t, o)
}
