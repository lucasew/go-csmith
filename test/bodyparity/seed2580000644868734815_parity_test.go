package bodyparity_test
import ("testing"; "csmith/pkg/csmith")
// Seed 2580000644868734815 (LevelC): Acc-late pure residual free-ref before residual free free-neither of same FE
// after residual free free-ref owner-only Acc-adjacent (func_1 g_247 of func_28 after g_716 before g_188). Session/FM-local.
func TestSeed2580000644868734815(t *testing.T) {
	o := csmith.Defaults(); o.Seed = 2580000644868734815
	assertOptsBodyParity(t, o)
}
