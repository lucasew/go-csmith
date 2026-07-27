package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 4775411138702774940 (LevelC): free-head mid solo pure-only Acc-late (func_64
// g_404 after free residual free-ref stream g_16/g_12/g_15) must FE-relative place
// before residual free successor on parent func_49. Free residual freeO-only mid
// free residual free-head before pure leaves Acc-late (seed639). Session/FM-local.
func TestSeed4775411138702774940(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 4775411138702774940
	assertOptsBodyParity(t, o)
}
