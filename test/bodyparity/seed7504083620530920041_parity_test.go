package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 7504083620530920041 (LevelC): nested pure FE head before own pure residual
// must not rotate free-ref own pure g_14 of func_1 past nested pure g_250 of
// func_17 (own residual free-ref on pure owner). AccEarlyExclusive must not delay
// correctly-early pure FE head g_140 of func_36 past free residual g_110.
// Session/FM-local — no package mutable state.
func TestSeed7504083620530920041(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 7504083620530920041
	assertOptsBodyParity(t, o)
}
