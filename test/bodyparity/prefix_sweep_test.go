package bodyparity_test

import (
	"fmt"
	"os"
	"testing"

	"csmith/pkg/csmith"
)

// BODYPARITY_PREFIX_SWEEP=1 — Defaults + --prefix-name × battery seeds and flag pairs.
func TestPrefixNameSweep(t *testing.T) {
	if os.Getenv("BODYPARITY_PREFIX_SWEEP") == "" {
		t.Skip("set BODYPARITY_PREFIX_SWEEP=1")
	}
	_ = upstreamCsmith(t)
	seeds := []uint64{0, 1, 2, 3, 4, 5, 7, 10, 42, 100, 123, 999, 12345, 145079}
	type mut struct {
		name string
		fn   func(*csmith.Options)
	}
	muts := []mut{
		{"alone", func(o *csmith.Options) {}},
		{"paranoid", func(o *csmith.Options) { o.Paranoid = true }},
		{"binary", func(o *csmith.Options) { o.BinaryConstant = true }},
		{"float", func(o *csmith.Options) { o.EnableFloat = true }},
		{"no_checksum", func(o *csmith.Options) { o.ComputeHash = false }},
		{"ccomp", func(o *csmith.Options) { o.CComp = true }},
		{"random_random", func(o *csmith.Options) { o.RandomRandom = true }},
		{"no_safe_math", func(o *csmith.Options) { o.SafeMath = false }},
		{"step_hash", func(o *csmith.Options) { o.StepHashByStmt = true }},
		{"access_once", func(o *csmith.Options) { o.AccessOnce = true }},
	}
	for _, seed := range seeds {
		for _, m := range muts {
			seed, m := seed, m
			t.Run(fmt.Sprintf("s%d_%s", seed, m.name), func(t *testing.T) {
				o := csmith.Defaults()
				o.Seed = seed
				o.PrefixName = true
				m.fn(&o)
				assertOptsBodyParity(t, o)
			})
		}
	}
}
