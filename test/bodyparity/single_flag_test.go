package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

func TestBodyParitySingleFlags(t *testing.T) {
	_ = upstreamCsmith(t)
	base := csmith.Defaults()
	base.Seed = 2
	// A few high-value CLI toggles (on if default off, or off if default on).
	cases := []struct {
		name string
		mut  func(*csmith.Options)
	}{
		{"klee", func(o *csmith.Options) { o.Klee = true }},
		{"crest", func(o *csmith.Options) { o.Crest = true }},
		{"no_jumps", func(o *csmith.Options) { o.Jumps = false }},
		{"no_volatiles", func(o *csmith.Options) { o.Volatiles = false }},
		{"no_pointers", func(o *csmith.Options) { o.Pointers = false }},
		{"no_arrays", func(o *csmith.Options) { o.Arrays = false }},
		{"no_safe_math", func(o *csmith.Options) { o.SafeMath = false }},
		{"no_bitfields", func(o *csmith.Options) { o.Bitfields = false }},
		{"no_checksum", func(o *csmith.Options) { o.ComputeHash = false }},
		{"step_hash", func(o *csmith.Options) { o.StepHashByStmt = true }},
		{"float", func(o *csmith.Options) { o.EnableFloat = true }},
		{"int128", func(o *csmith.Options) { o.Int128 = true }},
		{"max_funcs_3", func(o *csmith.Options) { o.MaxFuncs = 3 }},
		{"inline_function", func(o *csmith.Options) { o.InlineFunction = true }},
		{"concise", func(o *csmith.Options) { o.Concise = true }},
		{"nomain", func(o *csmith.Options) { o.NoMain = true }},
		{"math_notmp", func(o *csmith.Options) { o.MathNoTmp = true }},
		{"binary_constant", func(o *csmith.Options) { o.BinaryConstant = true }},
		{"no_structs", func(o *csmith.Options) { o.Structs = false }},
		{"no_unions", func(o *csmith.Options) { o.Unions = false }},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			o := base
			tc.mut(&o)
			assertOptsBodyParity(t, o)
		})
	}
}
