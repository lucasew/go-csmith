package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Regression: post_condition must use find_updated_final_facts only — falling
// back to non-final FindUpdatedFacts re-emitted a for-loop //assert(p_89…) that
// map_facts_*_final considered unchanged (seed-2 --paranoid --binary-constant).
func TestParanoidBinarySeed2(t *testing.T) {
	_ = upstreamCsmith(t)
	o := csmith.Defaults()
	o.Seed = 2
	o.Paranoid = true
	o.BinaryConstant = true
	assertOptsBodyParity(t, o)
}
