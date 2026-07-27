package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 4705676153835236351 (LevelC): pureOnly Acc-late solo pure FE head must not
// yank before residual free free-ref on parent (func_47 [g_904.f1,g_1151]; Acc
// g_1151…g_726.f0 g_904.f1). Session/FM-local — no package mutable state.
func TestSeed4705676153835236351(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 4705676153835236351
	assertOptsBodyParity(t, o)
}
