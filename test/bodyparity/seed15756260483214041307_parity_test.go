package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 15756260483214041307 (LevelC): pure multi-prefix func_12 [g_1648.f4,g_1638.f4]
// Acc-adjacent on func_1 after free residual free-ref g_3. Acc-early nested pure
// must not reverse multi FE order past Acc-adjacent pure multi mid when Acc-early
// before free residual g_1536 (UP: g_1648.f4 g_1638.f4). Session/FM-local.
func TestSeed15756260483214041307(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 15756260483214041307
	assertOptsBodyParity(t, o)
}
