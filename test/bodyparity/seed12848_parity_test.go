package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 12848 (fuzz): free-head func_59 residual free free-ref g_270 Acc-before pure
// residual g_528.f3 pure-only on intermediate FEs; reorderFreeHeadMidPure wrongly
// yanked pure before residual free free-ref owner. Gate: skip when residual free
// free-refs free-head owner. Session/FM-local.
func TestSeed12848(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 12848
	assertOptsBodyParity(t, o)
}
