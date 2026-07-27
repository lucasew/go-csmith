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
		{"prefix_name", func(o *csmith.Options) { o.PrefixName = true }},
		{"variable_attributes", func(o *csmith.Options) { o.VariableAttributes = true }},
		{"function_attributes", func(o *csmith.Options) { o.FunctionAttributes = true }},
		{"compatible_check", func(o *csmith.Options) { o.CompatibleCheck = true }},
		{"identify_wrappers", func(o *csmith.Options) { o.IdentifyWrappers = true }},
		// seed-2 was the remaining single-flag DIFF: SetupInOutMaps used MergeFacts
		// (new-first) instead of combine_facts/join_visits (old-first pointee order).
		{"paranoid", func(o *csmith.Options) { o.Paranoid = true }},
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
		{"no_global_variables", func(o *csmith.Options) { o.GlobalVariables = false }},
		{"no_addr_taken_of_locals", func(o *csmith.Options) { o.AddrTakenOfLocals = false }},
		// Forced globals under --no-addr-taken-of-locals even when --no-global-variables
		// (C++ RandomGlobalName assert is NDEBUG-no-op in Release golden).
		{"no_globals_no_addr_locals", func(o *csmith.Options) {
			o.GlobalVariables = false
			o.AddrTakenOfLocals = false
		}},
		// output_value_dump: no pointer dumps; itemized arrays use indices; unions
		// via itemize+is_field_readable (seed-2 and union arrays under --check-global).
		{"check_global", func(o *csmith.Options) { o.BlindCheckGlobal = true }},
		// Variable::hash uses Output() → ACCESS_ONCE when isAccessOnce.
		{"enable_access_once", func(o *csmith.Options) { o.AccessOnce = true }},
		// Crest type_to_string NDEBUG empty suffix for int128 (CREST_(x)).
		{"crest_uint128", func(o *csmith.Options) {
			o.Crest = true
			o.UInt128 = true
			o.Func1MaxParams = 5
		}},
		// Mid-gen Lhs→ArrayAccess cache froze ACCESS_ONCE before isAddrTaken
		// (seed-1 full pair is TestCrestAccessOnceSeed1).
		{"crest_access_once", func(o *csmith.Options) {
			o.Crest = true
			o.AccessOnce = true
		}},
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

// Seed that first exposed empty GlobalList under no-globals+no-addr-taken (type-attr campaign).
func TestBodyParityNoGlobalsNoAddrLocals(t *testing.T) {
	_ = upstreamCsmith(t)
	o := csmith.Defaults()
	o.Seed = 2155495818057410368
	o.GlobalVariables = false
	o.AddrTakenOfLocals = false
	assertOptsBodyParity(t, o)
}
