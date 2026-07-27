package bodyparity_test

import (
	"fmt"
	"testing"

	"csmith/pkg/csmith"
)

// Multi-seed sweep under --mark-mutable-const after int2str offset fix.
func TestMarkMutableConstSeedSweep(t *testing.T) {
	seeds := []uint64{
		0, 1, 2, 42, 100, 1000, 145079, 999999,
		123456789, 987654321, 5555555555, 7777777777,
	}
	for _, seed := range seeds {
		seed := seed
		t.Run(fmt.Sprintf("s%d", seed), func(t *testing.T) {
			opts := csmith.Defaults()
			opts.Seed = seed
			opts.MarkMutableConst = true
			assertOptsBodyParity(t, opts)
		})
	}
}
