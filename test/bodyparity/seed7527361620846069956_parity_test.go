package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 7527361620846069956 (LevelC): pure multi-prefix func_55 [g_116,g_87] under
// func_45/func_18. Acc-early mid pureOnly g_87 before free residual free-ref g_135;
// Acc-late multi head g_116 after free residual. Multi-prefix case2b places head
// immediately before free residual (g_87 g_116 g_135) — not case1 FE-order compact
// (g_116 g_87 g_135). Nested call tree. Session/FM-local — no package mutable state.
func TestSeed7527361620846069956(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 7527361620846069956
	assertOptsBodyParity(t, o)
}
