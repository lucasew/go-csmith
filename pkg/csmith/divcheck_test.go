package csmith

import (
	"context"
	"os"
	"runtime/debug"
	"testing"
)

// TestPureGenStrictResidual panics on residual sessOrAmbient(nil) under pureGenStrict.
// Opt-in: PURE_GEN_STRICT=1 (pollutes process tables mid-gen on residual hit).
func TestPureGenStrictResidual(t *testing.T) {
	if os.Getenv("PURE_GEN_STRICT") == "" {
		t.Skip("set PURE_GEN_STRICT=1 to chase residual ambient sessOrAmbient(nil)")
	}
	opts := Defaults()
	opts.Seed = 2
	ReinstallTestProcessSingletons()
	pureGenStrict = true
	defer func() {
		pureGenStrict = false
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
	t.Logf("pure gen ok len=%d", len(out))
}
