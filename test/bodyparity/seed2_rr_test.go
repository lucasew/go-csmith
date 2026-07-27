package bodyparity_test

import (
	"testing"
	"time"

	"csmith/pkg/csmith"
)

func TestSeed2RandomRandomParity(t *testing.T) {
	t0 := time.Now()
	o := csmith.Defaults()
	o.Seed = 2
	o.RandomRandom = true
	assertOptsBodyParity(t, o)
	t.Logf("elapsed %v", time.Since(t0))
}
