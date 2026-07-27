package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

func TestSeed50(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 50
	assertOptsBodyParity(t, o)
}
