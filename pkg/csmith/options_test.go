package csmith

import "testing"

// Defaults must match CGOptions.h CGOPTIONS_DEFAULT_* and
// CGOptions::set_default_settings (CGOptions.cpp).

func TestDefaultsMatchCGOptionsMacros(t *testing.T) {
	o := Defaults()
	// CGOptions.h inline constexpr defaults
	checks := []struct {
		name string
		got  int
		want int
	}{
		{"MaxFuncs", o.MaxFuncs, 10},
		{"MaxParams", o.MaxParams, 5},
		{"Func1MaxParams", o.Func1MaxParams, 3},
		{"MaxBlockSize", o.MaxBlockSize, 4},
		{"MaxBlockDepth", o.MaxBlockDepth, 5},
		{"MaxExprComplexity", o.MaxExprComplexity, 10}, // max_expr_depth
		{"MaxStructFields", o.MaxStructFields, 10},
		{"MaxUnionFields", o.MaxUnionFields, 5},
		{"MaxNestedStructLevel", o.MaxNestedStructLevel, 3},
		{"MaxPointerDepth", o.MaxPointerDepth, 5}, // max_indirect_level
		{"MaxArrayDim", o.MaxArrayDim, 3},
		{"MaxArrayLenPerDim", o.MaxArrayLenPerDim, 10},
		{"MaxArrayLength", o.MaxArrayLength, 256},
		{"MaxArrayNumInLoop", o.MaxArrayNumInLoop, 4},
		{"MaxExhaustiveDepth", o.MaxExhaustiveDepth, -1},
		{"MaxSplitFiles", o.MaxSplitFiles, 0},
		{"CoverageTestSize", o.CoverageTestSize, 500},
		{"InlineFunctionProb", o.InlineFunctionProb, 50},
		{"BuiltinFunctionProb", o.BuiltinFunctionProb, 50},
		{"ArrayOOBProb", o.ArrayOOBProb, 0},
		// CGOptions: null_pointer_dereference_prob / dead_pointer_dereference_prob only
		{"NullPointerDerefProb", o.NullPointerDerefProb, 0},
		{"DeadPointerDerefProb", o.DeadPointerDerefProb, 0},
		{"StopByStmt", o.StopByStmt, -1},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %d want %d (CGOptions default)", c.name, c.got, c.want)
		}
	}
}

func TestDefaultsBoolsMatchSetDefaultSettings(t *testing.T) {
	o := Defaults()
	// CGOptions::set_default_settings selected bools
	bools := []struct {
		name string
		got  bool
		want bool
	}{
		{"ComputeHash", o.ComputeHash, true},
		{"RandomBased", o.RandomBased, true},
		{"Structs", o.Structs, true},
		{"Unions", o.Unions, true},
		{"PackedStruct", o.PackedStruct, true},
		{"Bitfields", o.Bitfields, true},
		{"Math64", o.Math64, true},
		{"LongLong", o.LongLong, true},
		{"Int8", o.Int8, true},
		{"UInt8", o.UInt8, true},
		{"EnableFloat", o.EnableFloat, false},
		{"Pointers", o.Pointers, true},
		{"Arrays", o.Arrays, true},
		{"Jumps", o.Jumps, true},
		{"Volatiles", o.Volatiles, true},
		{"Consts", o.Consts, true},
		{"SafeMath", o.SafeMath, true}, // avoid_signed_overflow
		{"ForceGlobalsStatic", o.ForceGlobalsStatic, true},
		{"NoReturnDeadPointer", o.NoReturnDeadPointer, true},
		{"Int128", o.Int128, false},
		{"UInt128", o.UInt128, false},
		{"NoMain", o.NoMain, false},
	}
	for _, c := range bools {
		if c.got != c.want {
			t.Errorf("%s: got %v want %v", c.name, c.got, c.want)
		}
	}
}

func TestAllowInt64(t *testing.T) {
	// CGOptions::allow_int64
	// CGOptions.cpp:476–478 — !has_extension_support() && math64() && longlong()
	o := Defaults()
	if !o.AllowInt64() {
		t.Fatal("defaults should allow int64")
	}
	o.Math64 = false
	if o.AllowInt64() {
		t.Fatal("math64 false → no int64")
	}
	o = Defaults()
	o.LongLong = false
	if o.AllowInt64() {
		t.Fatal("longlong false → no int64")
	}
	o = Defaults()
	o.Klee = true
	if o.AllowInt64() {
		t.Fatal("klee extension → no int64")
	}
	o = Defaults()
	o.Crest = true
	if o.AllowInt64() {
		t.Fatal("crest extension → no int64")
	}
	o = Defaults()
	o.CoverageTest = true
	if o.AllowInt64() {
		t.Fatal("coverage-test extension → no int64")
	}
}

// TestSetDefaultSettingsParity locks CGOptions::set_default_settings field-by-field.
// CGOptions.cpp:215–322 — each assignment must match Defaults().
func TestSetDefaultSettingsParity(t *testing.T) {
	o := Defaults()
	bools := map[string]bool{
		"compute_hash": true, "fixed_struct_fields": false, "expand_struct": false,
		"allow_const_volatile": true, "avoid_signed_overflow": true,
		"paranoid": false, "quiet": false, "concise": false, "nomain": false,
		"random_based": true, "use_struct": true, "use_union": true,
		"compact_output": false, "klee": false, "crest": false, "ccomp": false,
		"coverage_test": false, "packed_struct": true, "bitfields": true,
		"prefix_name": false, "sequence_name_prefix": false, "compatible_check": false,
		"compound_assignment": true, "math64": true, "inline_function": false,
		"math_notmp": false, "longlong": true, "int8": true, "uint8": true,
		"enable_float": false, "strict_float": false, "pointers": true, "arrays": true,
		"strict_const_arrays": false, "jumps": true, "return_structs": true,
		"arg_structs": true, "return_unions": true, "arg_unions": true,
		"volatiles": true, "volatile_pointers": true, "const_pointers": true,
		"global_variables": true, "consts": true, "dangling_global_ptrs": true,
		"divs": true, "muls": true, "accept_argc": true, "step_hash_by_stmt": false,
		"const_as_condition": false, "match_exact_qualifiers": false,
		"blind_check_global": false, "no_return_dead_ptr": true,
		"hash_value_printf": true, "signed_char_index": true,
		"identify_wrappers": false, "mark_mutable_const": false,
		"force_globals_static": true, "force_non_uniform_array_init": true,
		"pre_incr_operator": true, "pre_decr_operator": true,
		"post_incr_operator": true, "post_decr_operator": true,
		"unary_plus_operator": true, "use_embedded_assigns": true,
		"use_comma_exprs": true, "take_union_field_addr": true,
		"vol_struct_union_fields": true, "const_struct_union_fields": true,
		"addr_taken_of_locals": true, "lang_cpp": false, "cpp11": false,
		"fast_execution": false, "Int128": false, "UInt128": false,
		"binary_constant": false,
	}
	got := map[string]bool{
		"compute_hash": o.ComputeHash, "fixed_struct_fields": o.FixedStructFields,
		"expand_struct": o.ExpandStruct, "allow_const_volatile": o.AllowConstVolatile,
		"avoid_signed_overflow": o.SafeMath, "paranoid": o.Paranoid, "quiet": o.Quiet,
		"concise": o.Concise, "nomain": o.NoMain, "random_based": o.RandomBased,
		"use_struct": o.Structs, "use_union": o.Unions, "compact_output": o.CompactOutput,
		"klee": o.Klee, "crest": o.Crest, "ccomp": o.CComp, "coverage_test": o.CoverageTest,
		"packed_struct": o.PackedStruct, "bitfields": o.Bitfields, "prefix_name": o.PrefixName,
		"sequence_name_prefix": o.SequenceNamePrefix, "compatible_check": o.CompatibleCheck,
		"compound_assignment": o.CompoundAssignment, "math64": o.Math64,
		"inline_function": o.InlineFunction, "math_notmp": o.MathNoTmp, "longlong": o.LongLong,
		"int8": o.Int8, "uint8": o.UInt8, "enable_float": o.EnableFloat, "strict_float": o.StrictFloat,
		"pointers": o.Pointers, "arrays": o.Arrays, "strict_const_arrays": o.StrictConstArrays,
		"jumps": o.Jumps, "return_structs": o.ReturnStructs, "arg_structs": o.ArgStructs,
		"return_unions": o.ReturnUnions, "arg_unions": o.ArgUnions, "volatiles": o.Volatiles,
		"volatile_pointers": o.VolatilePointers, "const_pointers": o.ConstPointers,
		"global_variables": o.GlobalVariables, "consts": o.Consts,
		"dangling_global_ptrs": o.DanglingGlobalPointers, "divs": o.Divs, "muls": o.Muls,
		"accept_argc": o.AcceptArgc, "step_hash_by_stmt": o.StepHashByStmt,
		"const_as_condition": o.ConstAsCondition, "match_exact_qualifiers": o.MatchExactQualifiers,
		"blind_check_global": o.BlindCheckGlobal, "no_return_dead_ptr": o.NoReturnDeadPointer,
		"hash_value_printf": o.HashValuePrintf, "signed_char_index": o.SignedCharIndex,
		"identify_wrappers": o.IdentifyWrappers, "mark_mutable_const": o.MarkMutableConst,
		"force_globals_static": o.ForceGlobalsStatic,
		"force_non_uniform_array_init": o.ForceNonUniformArrayInit,
		"pre_incr_operator": o.PreIncrOperator, "pre_decr_operator": o.PreDecrOperator,
		"post_incr_operator": o.PostIncrOperator, "post_decr_operator": o.PostDecrOperator,
		"unary_plus_operator": o.UnaryPlusOperator, "use_embedded_assigns": o.EmbeddedAssigns,
		"use_comma_exprs": o.CommaOperators, "take_union_field_addr": o.TakeUnionFieldAddr,
		"vol_struct_union_fields": o.VolStructUnionFields,
		"const_struct_union_fields": o.ConstStructUnionFields,
		"addr_taken_of_locals": o.AddrTakenOfLocals, "lang_cpp": o.LangCPP, "cpp11": o.CPP11,
		"fast_execution": o.FastExecution, "Int128": o.Int128, "UInt128": o.UInt128,
		"binary_constant": o.BinaryConstant,
	}
	for k, want := range bools {
		if got[k] != want {
			t.Errorf("%s: got %v want %v", k, got[k], want)
		}
	}
	ints := map[string]int{
		"max_funcs": 10, "max_params": 5, "max_block_size": 4, "max_blk_depth": 5,
		"max_expr_depth": 10, "max_struct_fields": 10, "max_union_fields": 5,
		"max_nested_struct_level": 3, "max_array_dimensions": 3,
		"max_array_length_per_dimension": 10, "max_array_length": 256,
		"max_exhaustive_depth": -1, "max_indirect_level": 5,
		"func1_max_params": 3, "coverage_test_size": 500, "max_array_num_in_loop": 4,
		"inline_function_prob": 50, "builtin_function_prob": 50,
		"null_pointer_dereference_prob": 0, "dead_pointer_dereference_prob": 0,
		"array_oob_prob": 0, "stop_by_stmt": -1,
	}
	gotI := map[string]int{
		"max_funcs": o.MaxFuncs, "max_params": o.MaxParams, "max_block_size": o.MaxBlockSize,
		"max_blk_depth": o.MaxBlockDepth, "max_expr_depth": o.MaxExprComplexity,
		"max_struct_fields": o.MaxStructFields, "max_union_fields": o.MaxUnionFields,
		"max_nested_struct_level": o.MaxNestedStructLevel, "max_array_dimensions": o.MaxArrayDim,
		"max_array_length_per_dimension": o.MaxArrayLenPerDim, "max_array_length": o.MaxArrayLength,
		"max_exhaustive_depth": o.MaxExhaustiveDepth, "max_indirect_level": o.MaxPointerDepth,
		"func1_max_params": o.Func1MaxParams, "coverage_test_size": o.CoverageTestSize,
		"max_array_num_in_loop": o.MaxArrayNumInLoop, "inline_function_prob": o.InlineFunctionProb,
		"builtin_function_prob": o.BuiltinFunctionProb,
		"null_pointer_dereference_prob": o.NullPointerDerefProb,
		"dead_pointer_dereference_prob": o.DeadPointerDerefProb,
		"array_oob_prob": o.ArrayOOBProb, "stop_by_stmt": o.StopByStmt,
	}
	for k, want := range ints {
		if gotI[k] != want {
			t.Errorf("%s: got %d want %d", k, gotI[k], want)
		}
	}
	if o.OutputPath != "" {
		t.Errorf("output_file default: got %q want empty", o.OutputPath)
	}
}

func TestHasConflictExtensionAndDelta(t *testing.T) {
	// CGOptions.cpp has_extension_conflict / has_delta_conflict / has_conflict subsets
	o := Defaults()
	if err := o.Validate(); err != nil {
		t.Fatalf("defaults must not conflict: %v", err)
	}
	o = Defaults()
	o.Klee, o.Crest = true, true
	if err := o.Validate(); err == nil {
		t.Fatal("klee+crest must conflict")
	}
	o = Defaults()
	o.DeltaMonitor, o.GoDelta = "m", "g"
	if err := o.Validate(); err == nil {
		t.Fatal("delta-monitor + go-delta must conflict")
	}
	o = Defaults()
	o.CPP11 = true
	o.LangCPP = false
	if err := o.Validate(); err == nil {
		t.Fatal("cpp11 without lang-cpp must conflict")
	}
	o = Defaults()
	o.SequenceNamePrefix = true // random_based default true
	if err := o.Validate(); err == nil {
		t.Fatal("sequence-name-prefix with random mode must conflict")
	}
	o = Defaults()
	o.Func1MaxParams = 9
	o.MaxParams = 3
	if err := o.Validate(); err == nil {
		t.Fatal("func1_max_params > max_params must conflict")
	}
}

func TestNormalizeUpstreamFlow(t *testing.T) {
	// fix_options_for_cpp + resolve_exhaustive_options side effects
	o := Defaults()
	o.LangCPP = true
	o = o.normalizeUpstreamFlow()
	if !o.MatchExactQualifiers || o.VolStructUnionFields || o.ConstStructUnionFields {
		t.Fatal("lang_cpp must force match_exact_qualifiers and clear vol/const struct fields")
	}
	o = Defaults()
	o.DFSExhaustive = true
	o = o.normalizeUpstreamFlow()
	if !o.FixedStructFields {
		t.Fatal("dfs_exhaustive must force fixed_struct_fields")
	}
	o = Defaults()
	o.FastExecution = true
	o = o.normalizeUpstreamFlow()
	if !o.LangCPP || o.Jumps {
		t.Fatal("fast_execution must enable lang_cpp and disable jumps")
	}
}
