package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 10563948572526880868 (LevelC): free-ref pure FE head g_9 of func_35 Acc has
// free residual g_2 g_8 then g_9 on func_1. Pure-prefix must not yank free-ref pure
// before free residual free-ref (same gate as seed1436 free-ref multi-prefix).
// Session/FM-local — no package mutable state.
func TestSeed10563948572526880868(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 10563948572526880868
	assertOptsBodyParity(t, o)
}
