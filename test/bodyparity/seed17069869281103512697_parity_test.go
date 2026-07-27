package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 17069869281103512697: pure FE head g_326 of func_101 Acc-late on parent
// func_89 residual — UP places g_326 before free residual g_387 (before g_256…);
// pure multi residual g_983… stays late after g_1003.
func TestSeed17069869281103512697(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 17069869281103512697
	assertOptsBodyParity(t, o)
}
