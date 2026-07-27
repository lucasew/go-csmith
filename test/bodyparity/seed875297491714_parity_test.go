package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 875297491714: multi pure-head func_60 residual free-ref free pure-only
// Acc invent (func_1 / func_45). Do not post-Acc invent Acc-missing pure residual
// free-ref free of multi pure-head (g_194). freeVal multi residual pure free-ref free
// of pure-head Acc-orders before free residual free-ref free of parent (g_36 before
// g_8). Multi residual pure free-ref free pure-only Acc-adjacent sibling invents after
// Acc predecessor (g_316 after g_266), not Acc-order before late Acc successor past
// free residual free-ref free of parent. Session-local — no package mutable state.
func TestSeed875297491714(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 875297491714
	assertOptsBodyParity(t, o)
}
