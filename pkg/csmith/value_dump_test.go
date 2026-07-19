package csmith

import (
	"strings"
	"testing"
)

func TestOutputValueDumpSimple(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	out := v.OutputValueDump("checksum ", 1, nil)
	// int may be %d or %lld depending on platform SizeInBytes
	if !strings.Contains(out, "checksum g_1 = %") || !strings.Contains(out, ", g_1);") {
		t.Fatal(out)
	}
}

func TestOutputValueDumpNilFieldHoleFailClosed(t *testing.T) {
	// Variable* always live in FieldVars sticky; soft invent skip would dump later fields
	ClearError()
	st := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	v := CreateVariableQfer("g_s", st, NewCVQualifiers([]bool{false}, []bool{false}))
	f0 := &Variable{Name: "g_s.f0", Type: GetIntType(), FieldVarOf: v}
	v.FieldVars = []*Variable{nil, f0}
	if s := v.OutputValueDump("checksum ", 1, nil); s != "" {
		t.Fatal("nil FieldVars hole must fail closed whole dump, not soft-skip", s)
	}
	if !HasError() {
		t.Fatal("nil FieldVars hole dump must SetError sticky")
	}
	ClearError()
	// union hole
	ut := &Type{isUnion: true, StructName: "U0", Fields: []StructField{
		{Name: "a", Type: GetIntType(), BitWidth: -1},
	}}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if uv == nil {
		t.Fatal("create union")
	}
	uv.FieldVars = []*Variable{nil}
	facts := []*FactUnion{MakeFactUnion(uv, 0)}
	if s := uv.OutputValueDump("c ", 1, facts); s != "" {
		t.Fatal("nil union FieldVars hole must fail closed", s)
	}
	if !HasError() {
		t.Fatal("nil union FieldVars hole dump must SetError sticky")
	}
	ClearError()
}

func TestOutputValueDumpStructFields(t *testing.T) {
	ClearError()
	st := &Type{
		isStruct: true, StructName: "S0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
			{Name: "f1", Type: GetSimpleType(EUInt), BitWidth: -1},
		},
	}
	v := CreateVariableQfer("g_s", st, NewCVQualifiers([]bool{false}, []bool{false}))
	out := v.OutputValueDump("checksum ", 1, nil)
	if !strings.Contains(out, "g_s.f0") || !strings.Contains(out, "g_s.f1") {
		t.Fatal(out)
	}
	// unsigned int may be %u or %llu depending on SizeInBytes
	if !strings.Contains(out, "%u") && !strings.Contains(out, "%llu") {
		t.Fatal("uint directive", out)
	}
}

func TestOutputValueDumpUnionReadable(t *testing.T) {
	ut := &Type{
		isUnion: true, StructName: "U0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
			{Name: "f1", Type: GetIntType(), BitWidth: -1},
		},
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	// no facts → nothing readable
	if s := uv.OutputValueDump("c ", 1, nil); s != "" {
		t.Fatal("empty facts should dump no union fields", s)
	}
	facts := []*FactUnion{MakeFactUnion(uv, 0)}
	out := uv.OutputValueDump("c ", 1, facts)
	if !strings.Contains(out, "g_u.f0") {
		t.Fatal(out)
	}
	if strings.Contains(out, "g_u.f1") {
		t.Fatal("f1 unreadable", out)
	}
}

func TestOutputValueDumpArrayExpand(t *testing.T) {
	v := &Variable{
		Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2},
		Qfer: NewCVQualifiers([]bool{false}, []bool{false}),
	}
	out := v.OutputValueDump("c ", 1, nil)
	if !strings.Contains(out, "g_a[0]") || !strings.Contains(out, "g_a[1]") {
		t.Fatal(out)
	}
}

func TestOutputValueDumpTypeNilSticky(t *testing.T) {
	// Variable + Type always live at dump; sticky no invent empty dump past Type-nil shell
	ClearError()
	if (&Variable{Name: "g_broken"}).OutputValueDump("c ", 0, nil) != "" {
		t.Fatal("Type-nil OutputValueDump must fail closed empty")
	}
	if !HasError() {
		t.Fatal("Type-nil OutputValueDump must SetError sticky")
	}
	ClearError()
	if outputValueDumpArray(&Variable{Name: "g_a", IsArray: true, ArraySizes: []int{2}}, "c ", 0, nil) != "" {
		t.Fatal("Type-nil outputValueDumpArray must fail closed empty")
	}
	if !HasError() {
		t.Fatal("Type-nil outputValueDumpArray must SetError sticky")
	}
	ClearError()
}

func TestExpandWithinRanges(t *testing.T) {
	got := expandWithinRanges([]int{2, 2})
	if len(got) != 4 {
		t.Fatal(len(got))
	}
	// util.cpp product: (0,0)(0,1)(1,0)(1,1)
	if got[0][0] != 0 || got[0][1] != 0 || got[3][0] != 1 || got[3][1] != 1 {
		t.Fatalf("order %v", got)
	}
	// util.cpp: zero size → empty expansion (no soft invent n=1)
	if expandWithinRanges([]int{2, 0}) != nil {
		t.Fatal("zero dim must not invent size 1")
	}
	if expandWithinRanges([]int{-1}) != nil {
		t.Fatal("negative dim must not invent size 1")
	}
	if expandWithinRanges(nil) != nil {
		t.Fatal("empty sizes")
	}
	// 1×3
	g13 := expandWithinRanges([]int{1, 3})
	if len(g13) != 3 || g13[2][0] != 0 || g13[2][1] != 2 {
		t.Fatalf("%v", g13)
	}
}

func TestBlindCheckGlobalMain(t *testing.T) {
	opts := Defaults()
	opts.BlindCheckGlobal = true
	opts.MaxFuncs = 1
	opts.MaxBlockSize = 1
	opts.MaxBlockDepth = 1
	g := NewProgramGenerator(opts)
	g.GenerateAllTypes()
	// seed simple global
	v := CreateVariableScalars("g_x", GetIntType(), false, false)
	g.VS.GlobalList = []*Variable{v}
	g.GenerateFunctions()
	main := g.OutputMain()
	if !strings.Contains(main, "checksum g_x") {
		t.Fatal(main)
	}
	// no platform_main / crc in blind path
	if strings.Contains(main, "platform_main_begin") {
		t.Fatal("blind path should skip platform begin", main)
	}
	// first call always live for non-empty Funcs
	if !strings.Contains(main, "func_") {
		t.Fatal("main must emit first function call", main)
	}
}

func TestOutputMainNoInventWithoutFirstInvoke(t *testing.T) {
	// OutputMgr.cpp:97 — MakeFuncInvocation always live; no invent main shell without call
	opts := Defaults()
	opts.NoMain = false
	g := NewProgramGenerator(opts)
	// incomplete first func (no RV/body) — BuildUserInvocation may still run with empty params
	g.Funcs.Funcs = []*Function{{
		Name: "func_1", ReturnType: GetIntType(),
		// no Body / not Built — invocation of unbuilt still possible for zero-param
		IsBuilt: true, BuildState: BuildBuilt,
	}}
	// nil Rng → BuildUserInvocation fails closed
	g.Rng = nil
	if out := g.OutputMain(); out != "" {
		t.Fatal("nil RNG first invoke must fail closed main", out)
	}
}
