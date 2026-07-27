package bodyparity_test
import ("testing"; "csmith/pkg/csmith")
// Seed 9838263505978394624 (fuzz): free-head pure residual pure-only Acc-early after pure residual free-ref
// of free-head FE before residual free free-ref owner — defer after pure residual pure-only Acc-late after residual free free-ref owner
// (func_12 g_276 after g_445 of free-head func_28; residual free free-ref owner g_260). Session/FM-local.
func TestSeed9838263505978394624(t *testing.T) {
	o := csmith.Defaults(); o.Seed = 9838263505978394624
	assertOptsBodyParity(t, o)
}
