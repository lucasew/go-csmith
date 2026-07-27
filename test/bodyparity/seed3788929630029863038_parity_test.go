package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 3788929630029863038 (LevelC): Path A Acc-late pure FE head before residual free
// must not reverse free residual free-ref on pure-IV owner (func_68 residual free g_7
// free-ref on owner before pure head g_75 on func_55). seed32 residual free g_1287 is
// not free-ref on pure owner — Path A still applies. Session/FM-local.
func TestSeed3788929630029863038(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 3788929630029863038
	assertOptsBodyParity(t, o)
}
