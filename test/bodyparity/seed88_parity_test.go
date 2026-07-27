package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 88: nested FE pure for-IV prefix (func_58 g_88.f0,g_657) must precede
// free-ref non-pure g_91 of the same FE on parent body map_stm.
func TestSeed88(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 88
	assertOptsBodyParity(t, o)
}
