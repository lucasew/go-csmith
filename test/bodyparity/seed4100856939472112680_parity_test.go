package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 4100856939472112680 (LevelC): pure residual free-ref free of pure-head func_58
// g_120 free-refs owner; Acc invent FE-rel after free residual free of pure-head g_34
// yanks early on func_46 (g_34 g_120 g_79…); UP Acc g_106 g_120 after free residual free
// stream. FE-rel after free residual free of pure-head for free residual pure free-ref
// free of pure-head free-ref free of owner requires Acc free residual free of pure-head
// immediately before pure; else Acc-order invent. case1 map skip still for residual free
// free-ref owner-only split (func_52 multi [g_120,g_716] residual free g_131). Session/FM-local.
func TestSeed4100856939472112680(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 4100856939472112680
	assertOptsBodyParity(t, o)
}
