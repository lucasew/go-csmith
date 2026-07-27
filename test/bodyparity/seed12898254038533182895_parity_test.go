package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 12898254038533182895 (LevelC): free residual free-ref g_300 Acc-early before
// pure multi-prefix of func_52 with mid pure g_84 before head g_72 on func_9.
// multi-prefix case2 must not FE-order-restore head first (UP: g_300 g_84 g_72).
// Session/FM-local.
func TestSeed12898254038533182895(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 12898254038533182895
	assertOptsBodyParity(t, o)
}
