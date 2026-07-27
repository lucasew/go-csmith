package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 57: pure multi free-ref head g_1597 pure-only on parent must pureMiss by prev
// when visit has pure multi free-ref mid g_1606 before head (prev has head first).
// Keep-visit pure-only nested pure FE head (seed1831) must not apply under pure multi
// free-ref visit invert. Path A / PureIVGlobals residual free pure-IV (g_114).
//
// Also pureMiss residual pure multi pure-only of pure-head: g_630.f1 after g_954 and
// g_1596 after g_1961.f3 — strip must not drop pureMiss early after parent free residual
// free not of pure-head FE (Acc late after g_1729.f3; invent skip pure residual pure-only
// multi). free-ref free residual pure of pure-head on owner (func_38 g_137/g_112 of
// func_70) must not strip/pack-last before free residual free g_168. PureMissTouched multi
// residual pure of pure multi keeps pureMiss map vs Acc yank (g_168 after g_137 g_112).
// Session/FM-local — no package mutable state.
func TestSeed57(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 57
	assertOptsBodyParity(t, o)
}
