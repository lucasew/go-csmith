package csmith

import (
	"context"
	"fmt"
	"os"
	"testing"
)

// TestPureGenStrictResidual asserts bag-local Generate does not write the
// quarantined testAmbientSession, and residual sessOrAmbient(nil) panics.
// Opt-in multi-seed battery: PURE_GEN_STRICT=1.
func TestPureGenStrictResidual(t *testing.T) {
	// Residual *Sess(nil) must not dual-fill ambient.
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("sessOrAmbient(nil) must panic")
			}
		}()
		_ = sessOrAmbient(nil)
	}()
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
			defer ReinstallTestProcessSingletons()
			s := NewSession(opts)
			// Marker on quarantined ambient: Generate must not write testAmbientSession.
			marker := 0xC0FFEE ^ int(seed)
			testAmbientSession.NextStmID = marker
			testAmbientSession.GenError = ErrSuccess
			// Poison ambient RNG so residual ProcessRng draws would desync body.
			testAmbientSession.Rng = NewRng(0xDEAD)
			out, err := s.Generate(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if out == "" {
				t.Fatal("empty program")
			}
			if testAmbientSession.NextStmID != marker {
				t.Fatalf("Generate mutated testAmbientSession.NextStmID: got %d want %d",
					testAmbientSession.NextStmID, marker)
			}
			if testAmbientSession.GenError != ErrSuccess {
				t.Fatalf("Generate residual sessNoteError(nil) polluted ambient GenError=%d",
					testAmbientSession.GenError)
			}
			// Re-generate with clean ambient: body must match (no ambient dependence).
			ReinstallTestProcessSingletons()
			again, err := NewSession(opts).Generate(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if again != out {
				t.Fatal("Generate body depends on ambient bag state")
			}
			t.Logf("pure gen ok seed=%d len=%d", seed, len(out))
		})
	}
}
