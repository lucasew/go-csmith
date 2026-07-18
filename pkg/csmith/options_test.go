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
}
