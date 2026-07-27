package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 9895936: nested pure for-IV FE head (func_32 g_1580@0) Acc-early-pollutes
// func_1 body map_stm (GO@5 vs UP@58). Post-body free-refing-callee FE anchor
// re-place defers Acc-head pure IVs only.
func TestSeed9895936(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 9895936
	assertOptsBodyParity(t, o)
}
