package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 22584: Acc-early nested pure FE heads (g_58, g_359.f0, g_258.f0) must
// sit next to their FE neighbors on parent body map_stm (after free-ref FE
// neighbor g_3; before non-free-ref g_529 / g_424). Multi-FE pure owners pick
// the Acc-early neighbor with max parent index (func_22 over func_30 for g_258.f0).
func TestSeed22584(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 22584
	assertOptsBodyParity(t, o)
}
