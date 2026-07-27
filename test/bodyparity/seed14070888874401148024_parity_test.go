package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 14070888874401148024: deeper pure multi-prefix func_63 [g_8,g_80] Acc-late
// after residual free g_105 on func_33/func_58. Multi-prefix case3 restores pure
// FE order; skip residual free pureIV Acc-early (seed875 g_70). Session/FM-local.
func TestSeed14070888874401148024(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 14070888874401148024
	assertOptsBodyParity(t, o)
}
