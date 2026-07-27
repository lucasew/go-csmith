package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 16294208416658607535 (LevelC stream_start=0): free-ref solo pure FE head
// g_1587 of func_6 Acc-late after residual free free-ref g_141 of same FE;
// UP: g_50 g_1587 g_141. pure-prefix free-ref solo + gapExempt fixes FE but
// yanks battery seed2/123 Acc order — leave open until mid-gen Acc matches.
// Also for (g_200){ if (g_200) } vs if (g_72) control-expr choose. Open.
func TestSeed16294208416658607535(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 16294208416658607535
	assertOptsBodyParity(t, o)
}
