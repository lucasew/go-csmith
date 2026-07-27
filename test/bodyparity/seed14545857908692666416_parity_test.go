package bodyparity_test

import "testing"

// Seed 14545857908692666416 (FuzzBodyParity): pure-IV Acc-early g_2522 of func_1
// was wrongly delayed after residual free free-ref g_2734 of pure-head func_9 by
// fixupAccEarlyPureHeadResidualFreeRefAfterResidualFree (no parent value free-ref).
// Gate: bodyValueFreeReadsVar. Session/FM-local — no package mutable state.
func TestSeed14545857908692666416(t *testing.T) {
	assertSeedBodyParity(t, 14545857908692666416)
}
