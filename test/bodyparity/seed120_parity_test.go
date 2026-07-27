package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 120: nested pure multi-prefix (func_53 g_28,g_59,g_63,g_66) on parent
// func_45 must keep pure-only g_59 before residual free g_5, not after.
func TestSeed120(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 120
	assertOptsBodyParity(t, o)
}
