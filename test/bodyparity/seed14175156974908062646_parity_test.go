package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 14175156974908062646: pure-only nested pure IVs of func_46 (g_147 FE
// head, g_527 mid) must sit at FE-relative slots on parent func_20 map_stm
// (g_147 before residual g_6; g_527 after g_166), not Acc-late after residual
// tail. func_1 inherits via callee FE merge.
func TestSeed14175156974908062646(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 14175156974908062646
	assertOptsBodyParity(t, o)
}
