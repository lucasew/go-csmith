package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 16629032864638926701 (LevelC): pure FE head g_128 of func_42 Acc-late on
// func_27/func_55 after residual free of intermediate placeFE (func_61). pureOnly
// relative walks nested call tree (not direct callees only) and Acc-late-gates vs
// placeFE residual free when owner residual free is absent on parent.
// Session/FM-local.
func TestSeed16629032864638926701(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 16629032864638926701
	assertOptsBodyParity(t, o)
}
