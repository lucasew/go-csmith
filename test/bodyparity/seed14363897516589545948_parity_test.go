package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 14363897516589545948 (LevelC): free residual free-ref g_38 Acc-early before
// free-ref pure multi-prefix g_39/g_65 of func_32. Pure-prefix must not yank free-ref
// pure FE head before free residual free-ref (UP: g_38 g_49 g_39 g_60 g_65).
// Session/FM-local.
func TestSeed14363897516589545948(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 14363897516589545948
	assertOptsBodyParity(t, o)
}
