package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 5139283748462763858: nested pure FE head g_261 of func_2 before own
// pure residual g_727 of func_1 on parent map_stm (not own pure before nested).
func TestSeed5139283748462763858(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 5139283748462763858
	assertOptsBodyParity(t, o)
}
