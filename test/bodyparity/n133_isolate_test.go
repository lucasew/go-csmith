package bodyparity_test

import (
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"csmith/pkg/csmith"
)

func TestN133Isolate(t *testing.T) {
	if os.Getenv("BODYPARITY_N133") == "" {
		t.Skip("BODYPARITY_N133=1")
	}
	_ = upstreamCsmith(t)
	raw, err := os.ReadFile("testdata/campfails/n133_s1569467152612809824.blob.hex")
	if err != nil {
		t.Fatal(err)
	}
	b, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	full := csmith.OptionsFromFuzzBlob(b).ForDropInParity()

	// progressive strip of exotic flags
	cases := []struct {
		name string
		mut  func(*csmith.Options)
	}{
		{"full", func(o *csmith.Options) {}},
		{"no_klee", func(o *csmith.Options) { o.Klee = false }},
		{"no_prefix", func(o *csmith.Options) { o.PrefixName = false }},
		{"no_compact", func(o *csmith.Options) { o.CompactOutput = false }},
		{"no_access_once", func(o *csmith.Options) { o.AccessOnce = false }},
		{"no_paranoid", func(o *csmith.Options) { o.Paranoid = false }},
		{"no_float", func(o *csmith.Options) { o.EnableFloat = false; o.StrictFloat = false }},
		{"only_paranoid_float_access", func(o *csmith.Options) {
			*o = csmith.Defaults()
			o.Seed = full.Seed
			o.Paranoid = true
			o.EnableFloat = true
			o.AccessOnce = true
		}},
		{"paranoid_float_access_prefix", func(o *csmith.Options) {
			*o = csmith.Defaults()
			o.Seed = full.Seed
			o.Paranoid = true
			o.EnableFloat = true
			o.AccessOnce = true
			o.PrefixName = true
		}},
		{"paranoid_only", func(o *csmith.Options) {
			*o = csmith.Defaults()
			o.Seed = full.Seed
			o.Paranoid = true
		}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			o := full
			tc.mut(&o)
			assertOptsBodyParity(t, o)
		})
	}
}
