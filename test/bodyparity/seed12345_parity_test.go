package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 12345 (fuzz 7fd3f664): UP g_120 g_157 g_156 g_167 vs GO g_157 g_156 g_167 g_120
// on parent reads — pure multi-prefix / Acc order of g_120 vs residual free cluster.
func TestSeed12345(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 12345
	assertOptsBodyParity(t, o)
}
