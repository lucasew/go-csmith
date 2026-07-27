package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 2048: pure FE-head Acc-late relative must not yank g_135 before g_446
// (func_66). Regression lock for fixupNestedFEPureOnlyRelativeOrder gate.
func TestSeed2048(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 2048
	assertOptsBodyParity(t, o)
}
