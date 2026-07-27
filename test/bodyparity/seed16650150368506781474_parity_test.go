package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 16650150368506781474 (LevelC): Acc-late pure FE head pure-only after residual free free-ref parent-only
// of same FE after free residual free-ref whose InitExpr is address-of pure (func_1 g_68 of func_63 after
// g_1091 InitExpr→g_68; residual free free-ref parent-only g_40). Session/FM-local.
func TestSeed16650150368506781474(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 16650150368506781474
	assertOptsBodyParity(t, o)
}
