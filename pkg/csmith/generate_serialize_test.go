package csmith

import (
	"context"
	"testing"
)

// Generate is sequential in-process (session-specific state, no generateMu).
// Concurrent calls in one process are unsupported — same as upstream csmith.
// Multi-seed sequential runs must not bleed: each Generate is a fresh session.
func TestGenerateSequentialMultiSeedIsolated(t *testing.T) {
	bodies := make([]string, 0, 8)
	for i := 0; i < 8; i++ {
		opts := Defaults()
		opts.Seed = uint64(10 + i)
		out, err := Generate(opts)
		if err != nil || out == "" {
			t.Fatalf("seed=%d err=%v empty=%v", opts.Seed, err, out == "")
		}
		bodies = append(bodies, out)
	}
	// Re-run seed 10: must match first emit (no sticky corruption from later seeds).
	opts := Defaults()
	opts.Seed = 10
	again, err := Generate(opts)
	if err != nil {
		t.Fatal(err)
	}
	if again != bodies[0] {
		t.Fatal("re-generate seed=10 diverged after multi-seed run (session bleed)")
	}
	// Distinct seeds should not all collapse to the same body.
	same := 0
	for i := 1; i < len(bodies); i++ {
		if bodies[i] == bodies[0] {
			same++
		}
	}
	if same == len(bodies)-1 {
		t.Fatal("all seeds produced identical body (RNG/session stuck)")
	}
}

// NewSession.Generate is the pure entry: explicit bag, never writes testAmbientSession.
func TestNewSessionGeneratePureEntry(t *testing.T) {
	opts := Defaults()
	opts.Seed = 2
	s := NewSession(opts)
	ambientBefore := testAmbientSession
	marker := 0xBEEF
	testAmbientSession.NextStmID = marker
	out, err := s.Generate(context.Background())
	if err != nil || out == "" {
		t.Fatalf("Generate: err=%v empty=%v", err, out == "")
	}
	if testAmbientSession != ambientBefore {
		t.Fatal("Generate must not replace testAmbientSession")
	}
	if testAmbientSession.NextStmID != marker {
		t.Fatalf("Generate mutated testAmbientSession.NextStmID: got %d want %d",
			testAmbientSession.NextStmID, marker)
	}
	if s.ProgramGen == nil || s.ProgramGen.Sess != s {
		t.Fatal("ProgramGenerator.Sess must point at the run session")
	}
	// Same session object + same opts should re-run cleanly (fresh internal tables via Initialize).
	out2, err := s.Generate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if out2 != out {
		t.Fatal("second Generate on same Session+opts diverged")
	}
	if testAmbientSession.NextStmID != marker {
		t.Fatal("second Generate mutated testAmbientSession")
	}
}
