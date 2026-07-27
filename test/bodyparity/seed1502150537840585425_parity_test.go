package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 1502150537840585425: non-func_1 free-head FE residual order for pure FE
// multi-prefix mid residual (func_32: g_49/g_370 of free-head func_65).
func TestSeed1502150537840585425(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 1502150537840585425
	assertOptsBodyParity(t, o)
}
