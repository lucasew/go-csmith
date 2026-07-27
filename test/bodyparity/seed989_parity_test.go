package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 989: pure FE head freeVal of nested pure-head that is own pure free residual
// pure free-ref free of free-head of parent must leave Acc-early put (func_59 g_99
// pure FE head of pure-head func_76 freeVal free-ref free of parent Acc-early after
// g_149 before g_161; freeVal pack after max free residual free of pure-head yanks
// late after g_205). fixupAccEarlyPureHeadFEPureResidualAfterFreeResidual skips
// isForIVOfFunc parent pure FE head freeVal. Session/FM-local — no package mutable state.
func TestSeed989(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 989
	assertOptsBodyParity(t, o)
}
