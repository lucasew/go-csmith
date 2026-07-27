package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// flagcamp n9: --prefix-name emptied assert identifiers in UP
// (assert ( == &);) while Go emitted full g_* names.
func TestPrefixNameAssertParity(t *testing.T) {
	opts := csmith.Defaults()
	opts.Seed = 2947185573186411220
	opts.AcceptArgc = false
	opts.Arrays = false
	opts.ComputeHash = false
	opts.CompoundAssignment = false
	opts.Divs = false
	opts.Muls = false
	opts.EmbeddedAssigns = false
	opts.CommaOperators = false
	opts.PreDecrOperator = false
	opts.PostDecrOperator = false
	opts.Jumps = false
	opts.LongLong = false
	opts.UInt8 = false
	opts.InlineFunction = true
	opts.Structs = false
	opts.ArgStructs = false
	opts.VolStructUnionFields = false
	opts.ConstStructUnionFields = false
	opts.Volatiles = false
	opts.VolatilePointers = false
	opts.GlobalVariables = false
	opts.AddrTakenOfLocals = false
	opts.StrictConstArrays = true
	opts.DanglingGlobalPointers = false
	opts.ForceGlobalsStatic = false
	opts.UInt128 = true
	opts.MathNoTmp = true
	opts.LabelAttributes = true
	opts.VariableAttributes = true
	opts.Paranoid = true
	opts.FixedStructFields = true
	opts.StrictVolatileRule = true
	opts.RandomRandom = true
	opts.BlindCheckGlobal = true
	opts.StepHashByStmt = true
	opts.MatchExactQualifiers = true
	opts.FreshArrayCtrlVarNames = true
	opts.IdentifyWrappers = true
	opts.StrictFloat = true
	opts.CompactOutput = true
	opts.PrefixName = true
	opts.Crest = true
	opts.NoDeltaReduction = true
	opts.FastExecution = true
	opts.HashValuePrintf = false
	opts.SignedCharIndex = false
	opts.MaxFuncs = 12
	opts.MaxBlockSize = 6
	opts.MaxBlockDepth = 8
	opts.MaxExprComplexity = 6
	opts.MaxStructFields = 12
	opts.MaxUnionFields = 7
	opts.MaxNestedStructLevel = 5
	opts.MaxPointerDepth = 6
	opts.MaxArrayDim = 2
	opts.MaxArrayLenPerDim = 8
	opts.MaxExhaustiveDepth = 8
	opts.InlineFunctionProb = 14
	opts.BuiltinFunctionProb = 68
	opts.ArrayOOBProb = 68
	opts.NullPointerDerefProb = 40
	opts.DeadPointerDerefProb = 7
	opts.StopByStmt = 61
	opts.CoverageTestSize = 37
	assertOptsBodyParity(t, opts)
}
