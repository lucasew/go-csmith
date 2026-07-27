package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 38656 + drop-in flags (FuzzBodyParity/8475e61345418de8): code body matched;
// func_63 feffect had Acc-late pure FE head g_76 of func_114 reordered early before
// residual free g_118 by summary-only solo pure-head Acc recovery. Gap free-refs were
// residual free of owner FE (not pollution). Gate: Acc-late reorder only on gap
// free-ref parent not residual free of pure-head owner. Session/FM-local.
func TestSeed38656FlagParity(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 38656
	o.TakeUnionFieldAddr = false
	o.ConstStructUnionFields = false
	o.StrictConstArrays = true
	o.Builtins = true
	o.SafeMath = false
	o.PackedStruct = false
	o.StepHashByStmt = true
	assertOptsBodyParity(t, o)
}
