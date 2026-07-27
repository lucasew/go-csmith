package bodyparity_test

import "testing"

// Seed 55 (FuzzBodyParity/6573b9b584d46b6e): free-head pure residual pure-only Acc-early
// g_139 of func_101 before residual free free-ref owner g_159. freehead pureonly defer
// requires FE-adjacent residual free free-ref owner (seed983 g_276|g_260). Residual free
// free-ref parent g_2 between pure and g_159 keeps Acc-early UP order. Session/FM-local.
func TestSeed55(t *testing.T) {
	assertSeedBodyParity(t, 55)
}
