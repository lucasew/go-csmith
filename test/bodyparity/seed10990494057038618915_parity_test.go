package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 10990494057038618915 (LevelC): Acc-early own pure residual free-ref g_1075 of
// pure-head func_59 after residual free free-ref g_942 (was Acc-early after g_100).
// fixupAccEarlyPureHeadResidualFreeRefAfterResidualFree. Session/FM-local.
func TestSeed10990494057038618915(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 10990494057038618915
	assertOptsBodyParity(t, o)
}
