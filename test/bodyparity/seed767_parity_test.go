package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 767: may-point compound RMW (*p_25)^= records g_716.f1 as both FE
// read and write. Acc→FE strip must not drop the read when Acc also writes
// the same field outer pure IV (func_1 for-IV g_716.f1).
func TestSeed767(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 767
	assertOptsBodyParity(t, o)
}
