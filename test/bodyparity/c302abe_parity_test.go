package bodyparity_test

import (
	"testing"
	"csmith/pkg/csmith"
)

func TestC302abe(t *testing.T) {
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
