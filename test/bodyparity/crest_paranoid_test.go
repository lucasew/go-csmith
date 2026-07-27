package bodyparity_test

import (
	"fmt"
	"testing"

	"csmith/pkg/csmith"
)

// Regression: gensym-key interleave put func_N_rv UW (key=N) before g_* asserts,
// gluing silent output_tab onto //assert (double indent) vs after onto `}`.
// factEmitSortKey ranks *_rv late (1<<29+N).
func TestCrestParanoidSeed1(t *testing.T) {
	_ = upstreamCsmith(t)
	o := csmith.Defaults()
	o.Seed = 1
	o.Crest = true
	o.Paranoid = true
	assertOptsBodyParity(t, o)
}

func TestCrestParanoidSeedSweep(t *testing.T) {
	_ = upstreamCsmith(t)
	for _, seed := range []uint64{0, 1, 2, 3, 42, 100, 123} {
		seed := seed
		t.Run(fmt.Sprintf("s%d", seed), func(t *testing.T) {
			o := csmith.Defaults()
			o.Seed = seed
			o.Crest = true
			o.Paranoid = true
			assertOptsBodyParity(t, o)
		})
	}
}
