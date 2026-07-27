package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 32: func_39 FE [g_381, g_1287, …] pure FE head Acc-late after residual free
// free-ref g_1287; UP: g_381 g_1287. Fixed by FixupAllAccLatePureFEHeadsAfterAllFuncs
// (post-all-bodies Path A with complete PureIVGlobals). Mid-gen Path A deferred.
// Session-local — no package globals.
func TestSeed32(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 32
	assertOptsBodyParity(t, o)
}
