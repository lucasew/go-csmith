package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 12593: free-head nested FE func_30 [g_2249…g_2252, g_1678, g_3964] —
// pure residual pure for-IV g_1678 Acc-late after residual free g_3964 on
// parent FE (func_1 / intermediate). UP: g_2252 g_1678 g_3964.
// Fix: reorderFreeHeadMidPureBeforeResidualFree on summary (session-local;
// no package globals). FE-adjacent fragment gate + free-ref-outside-owner skip.
func TestSeed12593(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 12593
	assertOptsBodyParity(t, o)
}
