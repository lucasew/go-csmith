package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 11647790213653658898 (LevelC): free-head func_41 pure residual g_225 Acc-late
// on func_1 after free residual free-ref g_207 and residual free g_493 of same
// free-head FE. Free residual free-ref mid free-head FE: place pure residual
// pure-only before residual free after last free residual free-ref currently
// before pure (UP: g_207 g_225 g_493 g_116 g_182). Session/FM-local.
func TestSeed11647790213653658898(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 11647790213653658898
	assertOptsBodyParity(t, o)
}
