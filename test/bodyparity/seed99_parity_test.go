package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 99: free residual pure free-ref free of free-head freefRef Acc-late after
// free residual free of free-head must not yank own pure residual pure free-ref
// free of pure-head (func_7 g_541 pure residual pure free-ref free of pure-head
// late before g_700; free residual pure free-only of free-head func_40 FE[1];
// free residual free of free-head g_766 then residual free g_110 Acc-early
// without pure). fixupFreeHeadFEPureResidualRelativeOrder skips isForIVOfFunc
// parent. Session/FM-local — no package mutable state.
func TestSeed99(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 99
	assertOptsBodyParity(t, o)
}
