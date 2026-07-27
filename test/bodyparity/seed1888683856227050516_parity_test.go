package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 1888683856227050516 (LevelC): own pure FE head g_163 of func_7 Acc-late on
// func_1 after free residual free-ref g_116 that is also residual of sibling
// func_22. ownPureNestedFEHead must not place pure before sibling-shared free
// residual free-ref (Acc g_113 g_116…g_178 g_163 matches UP). seed8105 exclusive
// free residual free-ref of owner FE only still pure-before-free. Session/FM-local.
func TestSeed1888683856227050516(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 1888683856227050516
	assertOptsBodyParity(t, o)
}
