package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 773070767187810853 (LevelC): pure multi-prefix head g_2.f3 of func_3 is
// exclusive residual Acc-late on func_1 after shared free residual g_532… of
// func_34 and free-ref g_181. Multi-prefix case1 must not compact exclusive pure
// multi head before sibling-shared residual (Acc order already correct).
// Session/FM-local — no package mutable state.
func TestSeed773070767187810853(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 773070767187810853
	assertOptsBodyParity(t, o)
}
