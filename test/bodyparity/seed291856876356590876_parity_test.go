package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 291856876356590876: Acc-early pure FE head g_182.f4 of func_2 after
// sibling free residual (func_14), before exclusive residual g_277.f4.
func TestSeed291856876356590876(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 291856876356590876
	assertOptsBodyParity(t, o)
}
