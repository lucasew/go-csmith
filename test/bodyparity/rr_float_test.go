package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// random_random equal-group flip order for simple types must follow ProbName
// map order (pULongLong before pFloat), not eSimpleType (eFloat before eULongLong).
func TestRandomRandomFloatParity(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 626407418334045425
	o.RandomRandom = true
	o.EnableFloat = true
	assertOptsBodyParity(t, o)
}
