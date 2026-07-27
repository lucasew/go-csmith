package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

func TestIsolateCrest(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 8048476871572835687
	o.Crest = true
	o.Func1MaxParams = 2
	assertOptsBodyParity(t, o)
}

func TestIsolateRandomRandom(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 8048476871572835687
	o.RandomRandom = true
	assertOptsBodyParity(t, o)
}

func TestIsolateCrestRandomRandom(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 8048476871572835687
	o.Crest = true
	o.RandomRandom = true
	o.Func1MaxParams = 2
	assertOptsBodyParity(t, o)
}

func TestIsolateKleeRandomRandom(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 8048476871572835687
	o.Klee = true
	o.RandomRandom = true
	o.Func1MaxParams = 2
	assertOptsBodyParity(t, o)
}
