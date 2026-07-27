package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// flagcamp n73: --prefix-name + --enable-access-once checksum dump must use
// to_string → Output → ACCESS_ONCE(), not bare empty name omitting the value arg.
func TestAccessOncePrefixChecksumDump(t *testing.T) {
	opts := csmith.Defaults()
	opts.Seed = 5026504309344998759
	opts.Arrays = false
	opts.Bitfields = false
	opts.ComputeHash = false
	opts.CompoundAssignment = false
	opts.Consts = false
	opts.EmbeddedAssigns = false
	opts.CommaOperators = false
	opts.PreIncrOperator = false
	opts.PreDecrOperator = false
	opts.UnaryPlusOperator = false
	opts.Int8 = false
	opts.UInt8 = false
	opts.EnableFloat = true
	opts.InlineFunction = true
	opts.Pointers = false
	opts.Structs = false
	opts.ReturnStructs = false
	opts.ArgStructs = false
	opts.ArgUnions = false
	opts.ConstStructUnionFields = false
	opts.ConstPointers = false
	opts.AddrTakenOfLocals = false
	opts.DanglingGlobalPointers = false
	opts.Builtins = true
	opts.ForceNonUniformArrayInit = false
	opts.BinaryConstant = true
	opts.MathNoTmp = true
	opts.FunctionAttributes = true
	opts.VariableAttributes = true
	opts.PackedStruct = false
	opts.Paranoid = true
	opts.AccessOnce = true
	opts.RandomRandom = true
	opts.BlindCheckGlobal = true
	opts.MatchExactQualifiers = true
	opts.FreshArrayCtrlVarNames = true
	opts.CompactOutput = true
	opts.PrefixName = true
	opts.CompatibleCheck = true
	opts.CoverageTest = true
	opts.Concise = true
	opts.Quiet = true
	opts.LangCPP = true
	opts.CPP11 = true
	opts.FastExecution = true
	opts.NoReturnDeadPointer = false
	opts.HashValuePrintf = false
	opts.MaxFuncs = 14
	opts.MaxBlockSize = 3
	opts.MaxBlockDepth = 6
	opts.MaxExprComplexity = 1
	opts.MaxStructFields = 5
	opts.MaxUnionFields = 7
	opts.MaxPointerDepth = 3
	opts.MaxArrayDim = 5
	opts.MaxArrayLenPerDim = 12
	opts.MaxExhaustiveDepth = 2
	opts.InlineFunctionProb = 27
	opts.BuiltinFunctionProb = 81
	opts.ArrayOOBProb = 57
	opts.NullPointerDerefProb = 52
	opts.DeadPointerDerefProb = 68
	opts.StopByStmt = 13
	opts.CoverageTestSize = 17
	opts.IntSize = 2
	assertOptsBodyParity(t, opts)
}
