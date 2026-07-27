package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 0 + heavy flag surface (fuzz c302abe8): pure multi-prefix-only FE
// func_55 [g_328.f2,g_109] Acc-late pure multi-prefix head on func_34 — multi-prefix
// case2 must not yank head before other-path free residual when FE has no residual
// after pure multi-prefix (unlike seed875). Session/FM-local.
func TestSeed0VarAttrsParity(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 0
	o.EnableFloat = true
	o.InlineFunction = true
	o.ReturnStructs = false
	o.ArgStructs = false
	o.ArgUnions = false
	o.TakeUnionFieldAddr = false
	o.VolStructUnionFields = false
	o.ConstStructUnionFields = false
	o.ConstPointers = false
	o.GlobalVariables = false
	o.AddrTakenOfLocals = false
	o.DanglingGlobalPointers = false
	o.Int128 = true
	o.UInt128 = true
	o.BinaryConstant = true
	o.FunctionAttributes = true
	o.LabelAttributes = true
	o.VariableAttributes = true
	o.PackedStruct = false
	o.Paranoid = true
	o.AccessOnce = true
	assertOptsBodyParity(t, o)
}
