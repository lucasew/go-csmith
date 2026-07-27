package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 28465 (fuzz 5fc6dcb1 "1o"):
//  1) Acc-early-before: keep pure when next follows pure in a nested FE
//     (g_98.f4 mid residual of free-head func_45 before g_265; firstRes of
//     owner func_37 is non-free-ref g_286.f2 — do not yank over free-refs).
//  2) multi-prefix case2: do not anchor on same-FE residual free (g_283.f0 of
//     func_14) — UP keeps Acc-late pure multi-prefix head g_163 after g_620.
//  3) Missing pure multi FE head Acc invent after Acc-adj Acc pred (g_163 after
//     g_620), not only before late Acc succ g_286.
//  4) pure residual pure of free-head freeSyn/pure-only Acc-early before Acc pred
//     Acc-order after Acc pred (g_506.f0 after g_178; Acc-early before g_213.f1).
//  5) free residual free of pure-head Acc-adj after pure multi FE head pure-only
//     of same FE (g_286/g_624 of func_14 after g_163*; chain after free residual
//     pure Acc-order of free-head cluster g_540/g_551 of func_28 after g_506.f0).
//     free residual free of pure-head pass runs after free residual pure of free-
//     head Acc-order so Acc pred is late Acc-adj. Session/FM-local — no package
//     mutable state.
func TestSeed28465(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 28465
	assertOptsBodyParity(t, o)
}
