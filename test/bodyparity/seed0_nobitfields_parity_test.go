package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 0 --no-bitfields: Acc-early pure FE head g_829.f0 of func_35 on parent
// body map_stm (GO early next to free residual g_858; UP late after residual free).
func TestSeed0NoBitfields(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 0
	o.Bitfields = false
	assertOptsBodyParity(t, o)
}
