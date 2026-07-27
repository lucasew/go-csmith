package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 15934573825443220977: pure-prefix g_150 of func_12 before free residual
// free-ref g_250 on func_1. Exclusive residual must not reverse pure-prefix when
// mid-gen Acc already correct (PurePrefixMoved unset). Session/FM-local.
func TestSeed15934573825443220977(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 15934573825443220977
	assertOptsBodyParity(t, o)
}
