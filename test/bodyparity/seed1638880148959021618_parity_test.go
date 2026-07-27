package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 1638880148959021618: pure residual pure free-ref free of free-head func_9
// (g_254 after g_41) that is also solo pure FE head of pure-head deeper func_13
// must FE-rel invent early, not Acc-append late (deeperPureHeadSolo pure-only path).
func TestSeed1638880148959021618(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 1638880148959021618
	assertOptsBodyParity(t, o)
}
