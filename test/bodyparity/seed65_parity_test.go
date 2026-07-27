package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 65: own pure freeVal free-ref free of free-head parent that is pure residual
// pure free-only of free-head nested Acc-early after free residual free of free-head
// intermediate — Acc invent RELOC after last enclosing global pure IV, before first
// non-field parent freeVal free-ref free after a field parent freeVal free-ref free
// (g_330.f0 then g_53 then g_397). Session/FM-local — no package mutable state.
func TestSeed65(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 65
	assertOptsBodyParity(t, o)
}
