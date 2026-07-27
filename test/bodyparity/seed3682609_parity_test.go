package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 3682609: nested pure FE head g_1818.f1 of func_2 before own pure
// residual g_2467.f0 of func_1 on parent map_stm.
func TestSeed3682609(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 3682609
	assertOptsBodyParity(t, o)
}
