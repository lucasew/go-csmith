package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 6689290986788227625 (LevelC):
//   - func_1 pureMiss strip: own pure-only free-ref only deep (g_205) / nested pure
//     FE head free-ref non-owner (g_645) — UP drops from FE reads. Keep free-ref on
//     direct pure-IV owner (seed48 g_1495).
//   - func_10 Acc invent: solo pure FE head pure-only already Acc-late past free residual
//     free-ref parent gap → reorder before first residual free of owner FE (g_645
//     before g_748). Not pure multi (seed57), not free-ref residual free parent/owner
//     (seed470/123), not gap without free residual free-ref parent (seed639).
// Session/FM-local — no package mutable state.
func TestSeed6689290986788227625(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 6689290986788227625
	assertOptsBodyParity(t, o)
}
