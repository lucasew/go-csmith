package bodyparity_test

import (
	"fmt"
	"testing"
	"csmith/pkg/csmith"
)

func TestHardRRNoPtrParity(t *testing.T) {
	_ = upstreamCsmith(t)
	seeds := []uint64{0, 1, 2, 3, 4, 5, 7, 10, 42}
	for _, seed := range seeds {
		for _, mf := range []int{2, 3} {
			name := fmt.Sprintf("s%d_mf%d", seed, mf)
			t.Run(name, func(t *testing.T) {
				o := csmith.Defaults()
				o.Seed = seed
				o.Pointers = false
				o.RandomRandom = true
				o.MaxFuncs = mf
				assertOptsBodyParity(t, o)
			})
		}
	}
}
