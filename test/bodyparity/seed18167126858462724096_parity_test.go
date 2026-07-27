package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 18167126858462724096 (fuzz 6ee744e0): pure multi-prefix FE head g_449 of
// func_20 Acc-late after residual free g_409 on func_1. UP: g_449 g_409.
func TestSeed18167126858462724096(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 18167126858462724096
	assertOptsBodyParity(t, o)
}
