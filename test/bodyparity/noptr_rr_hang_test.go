package bodyparity_test

import (
	"context"
	"testing"
	"time"

	"csmith/pkg/csmith"
)

// Regression: FindParentBlockOfStmID tree-walk was O(n)/lookup under
// --random-random --no-pointers (seed 1), hanging goGenTimeout. Function-local
// stmParentIdx (C++ Statement::parent) keeps generation finite.
// Body parity may still diverge (eligible-set n drift at ChooseOKVar); hang is separate.
func TestNoPtrRandomRandomSeed1Completes(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 1
	o.Pointers = false
	o.RandomRandom = true
	o.MaxFuncs = 3
	o = o.ForDropInParity()
	o.Argv = o.CLIArgs()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	t0 := time.Now()
	out, err := csmith.GenerateContext(ctx, o)
	if err != nil {
		t.Fatalf("generate: %v after %s", err, time.Since(t0))
	}
	if len(out) < 100 {
		t.Fatalf("tiny output %d after %s", len(out), time.Since(t0))
	}
	t.Logf("ok bytes=%d elapsed=%s", len(out), time.Since(t0))
}
