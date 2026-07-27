package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 16910086269757601142 (LevelC): free-head func_69 FE mid pure multi
// [g_117,g_91] free-ref on parent func_41. Acc-late pure multi mid g_91 after
// residual free g_128 of free-head FE. Mid free-head pure multi residual relative
// places g_91 before g_128 (UP: g_117 g_91 g_128). Free residual FE head may be
// absent on parent. Session/FM-local — no package mutable state.
func TestSeed16910086269757601142(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 16910086269757601142
	assertOptsBodyParity(t, o)
}
