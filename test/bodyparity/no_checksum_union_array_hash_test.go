package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// flagcamp n62: --no-checksum + union array hash opens index loops but must not
// sink subfields (ArrayVariable.cpp:791–798 only sinks eSimple elements).
func TestNoChecksumUnionArrayHashEmptyBody(t *testing.T) {
	opts := csmith.Defaults()
	opts.Seed = 17844745835742568789
	opts.Bitfields = false
	opts.ComputeHash = false
	opts.Consts = false
	opts.Divs = false
	opts.PreDecrOperator = false
	opts.UnaryPlusOperator = false
	opts.Jumps = false
	opts.LongLong = false
	opts.UInt8 = false
	opts.Math64 = false
	opts.Pointers = false
	opts.Structs = false
	opts.ReturnStructs = false
	opts.ArgStructs = false
	opts.TakeUnionFieldAddr = false
	opts.Builtins = true
	opts.ForceGlobalsStatic = false
	opts.ForceNonUniformArrayInit = false
	opts.Int128 = true
	opts.UInt128 = true
	opts.BinaryConstant = true
	opts.FunctionAttributes = true
	opts.LabelAttributes = true
	opts.VariableAttributes = true
	opts.Paranoid = true
	opts.StrictVolatileRule = true
	opts.StepHashByStmt = true
	opts.MarkMutableConst = true
	opts.StrictFloat = true
	opts.CompatibleCheck = true
	opts.CComp = true
	opts.FastExecution = true
	opts.NoReturnDeadPointer = false
	opts.MaxFuncs = 3
	opts.MaxBlockSize = 2
	opts.MaxBlockDepth = 3
	opts.MaxExprComplexity = 12
	opts.MaxStructFields = 8
	opts.MaxPointerDepth = 2
	opts.MaxArrayDim = 5
	opts.MaxArrayLenPerDim = 4
	opts.MaxExhaustiveDepth = 2
	opts.InlineFunctionProb = 92
	opts.BuiltinFunctionProb = 70
	opts.ArrayOOBProb = 9
	opts.NullPointerDerefProb = 22
	opts.DeadPointerDerefProb = 5
	opts.StopByStmt = 46
	opts.CoverageTestSize = 29
	assertOptsBodyParity(t, opts)
}
