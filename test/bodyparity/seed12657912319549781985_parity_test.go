package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 12657912319549781985 (LevelC): pure multi-prefix func_10 [g_243.f3,g_1481.f3].
// Acc-early must not yank pure multi head past mid pure when first residual free of
// same FE free-refs on the callee (keeps FE multi-prefix order). seed875 first residual
// free is not free-ref on callee — Acc-early may still reverse multi-prefix mid/head.
// Session/FM-local.
func TestSeed12657912319549781985(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 12657912319549781985
	assertOptsBodyParity(t, o)
}
