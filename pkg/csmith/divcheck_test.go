package csmith

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"testing"
)

// TestPureGenStrictResidual panics on residual sessOrAmbient(nil) under pureGenStrict.
// Generate itself enables pureGenStrict; this probe also re-asserts multi-seed.
// Opt-in multi-seed battery: PURE_GEN_STRICT=1 (extra seeds beyond always-on Generate lock).
func TestPureGenStrictResidual(t *testing.T) {
	seeds := []uint64{2}
	if os.Getenv("PURE_GEN_STRICT") != "" {
		seeds = []uint64{1, 2, 3, 7, 65, 123, 353}
	}
	for _, seed := range seeds {
		seed := seed
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			opts := Defaults()
			opts.Seed = seed
			ReinstallTestProcessSingletons()
			// Generate enables pureGenStrict; also force here for mid-test clarity.
			prev := pureGenStrict
			pureGenStrict = true
			defer func() {
				pureGenStrict = prev
				ReinstallTestProcessSingletons()
			}()
			s := NewSession(opts)
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%v\n%s", r, debug.Stack())
				}
			}()
			out, err := s.Generate(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if out == "" {
				t.Fatal("empty program")
			}
			if activeSession != nil {
				t.Fatal("Generate must not leave activeSession installed")
			}
			t.Logf("pure gen ok seed=%d len=%d", seed, len(out))
		})
	}
}
