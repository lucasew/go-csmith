package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 3892248577 (fuzz 2deb0c61): pure multi-prefix func_11 [g_253,g_24,g_349]
// before residual free free-ref g_1161. Acc-early after free-ref must not yank
// pure multi head past same-FE residual free free-ref (parent-only free-ref).
// Session/FM-local.
func TestSeed3892248577(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 3892248577
	assertOptsBodyParity(t, o)
}
