package bodyparity_test

import (
	"fmt"
	"testing"

	"csmith/pkg/csmith"
)

func TestCrestAccessOnceSeedSweep(t *testing.T) {
	_ = upstreamCsmith(t)
	for _, seed := range []uint64{0, 1, 2, 3, 4, 5, 7, 10, 42, 100, 123, 999, 145079} {
		seed := seed
		t.Run(fmt.Sprintf("s%d", seed), func(t *testing.T) {
			o := csmith.Defaults()
			o.Seed = seed
			o.Crest = true
			o.AccessOnce = true
			assertOptsBodyParity(t, o)
		})
	}
}
