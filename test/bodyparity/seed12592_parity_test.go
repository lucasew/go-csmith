package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 12592 (fuzz 285acbaf): pure multi-prefix func_56 [g_292,g_121].
// Parent free-ref of one multi-prefix member must not pureOnlyRel / free-head-mid
// yank the pure-only sibling early (func_21 g_292; func_51 g_121). Session-local.
func TestSeed12592(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 12592
	assertOptsBodyParity(t, o)
}
