package csmith

import (
	"strings"
	"testing"
)

func TestOutputValueDumpSimple(t *testing.T) {
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntType(), false, false)
	out := v.OutputValueDumpSess(testAmbientSession, "checksum ", 1, nil)
	// int may be %d or %lld depending on platform SizeInBytes
	if !strings.Contains(out, "checksum g_1 = %") || !strings.Contains(out, ", g_1);") {
		t.Fatal(out)
	}
}

func TestOutputValueDumpNilFieldHoleFailClosed(t *testing.T) {
	// Variable* always live in FieldVars sticky; soft invent skip would dump later fields
	ClearErrorSess(testAmbientSession)
	st := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	v := CreateVariableQferSess(testAmbientSession, "g_s", st, NewCVQualifiers([]bool{false}, []bool{false}))
	f0 := &Variable{Name: "g_s.f0", Type: GetIntType(), FieldVarOf: v}
	v.FieldVars = []*Variable{nil, f0}
	if s := v.OutputValueDumpSess(testAmbientSession, "checksum ", 1, nil); s != "" {
		t.Fatal("nil FieldVars hole must fail closed whole dump, not soft-skip", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FieldVars hole dump must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// union hole
	ut := &Type{isUnion: true, StructName: "U0", Fields: []StructField{
		{Name: "a", Type: GetIntType(), BitWidth: -1},
	}}
	uv := CreateVariableQferSess(testAmbientSession, "g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if uv == nil {
		t.Fatal("create union")
	}
	uv.FieldVars = []*Variable{nil}
	facts := []*FactUnion{MakeFactUnion(uv, 0)}
	if s := uv.OutputValueDumpSess(testAmbientSession, "c ", 1, facts); s != "" {
		t.Fatal("nil union FieldVars hole must fail closed", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil union FieldVars hole dump must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputValueDumpStructFields(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	st := &Type{
		isStruct: true, StructName: "S0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
			{Name: "f1", Type: GetSimpleType(EUInt), BitWidth: -1},
		},
	}
	v := CreateVariableQferSess(testAmbientSession, "g_s", st, NewCVQualifiers([]bool{false}, []bool{false}))
	out := v.OutputValueDumpSess(testAmbientSession, "checksum ", 1, nil)
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
	uv := CreateVariableQferSess(testAmbientSession, "g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	// no facts → nothing readable
	if s := uv.OutputValueDumpSess(testAmbientSession, "c ", 1, nil); s != "" {
		t.Fatal("empty facts should dump no union fields", s)
	}
	facts := []*FactUnion{MakeFactUnion(uv, 0)}
	out := uv.OutputValueDumpSess(testAmbientSession, "c ", 1, facts)
	if !strings.Contains(out, "g_u.f0") {
		t.Fatal(out)
	}
	if strings.Contains(out, "g_u.f1") {
		t.Fatal("f1 unreadable", out)
	}
	// incomplete UnionFacts residual: soft invent was soft-skip unreadable then partial dump.
	// Fair: sticky fail closed empty whole dump.
	ClearErrorSess(testAmbientSession)
	if s := uv.OutputValueDumpSess(testAmbientSession, "c ", 1, IncompleteUnionFactSlice()); s != "" {
		t.Fatal("incomplete UnionFacts OutputValueDump must fail closed empty", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete UnionFacts OutputValueDump must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputValueDumpArrayExpand(t *testing.T) {
	// live AsArray required for virtual expand (no invent scalar dump past IsArray shell)
	av := &ArrayVariable{
		Variable: Variable{
			Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2},
			Qfer: NewCVQualifiers([]bool{false}, []bool{false}),
		},
		Sizes: []int{2},
	}
	av.AsArray = av
	out := av.Variable.OutputValueDumpSess(testAmbientSession, "c ", 1, nil)
	if !strings.Contains(out, "g_a[0]") || !strings.Contains(out, "g_a[1]") {
		t.Fatal(out)
	}
	// IsArray without AsArray soft invent was scalar printf path
	ClearErrorSess(testAmbientSession)
	shell := &Variable{Name: "g_b", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	if shell.OutputValueDumpSess(testAmbientSession, "c ", 1, nil) != "" {
		t.Fatal("IsArray without AsArray OutputValueDump must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray OutputValueDump must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputValueDumpTypeNilSticky(t *testing.T) {
	// Variable + Type always live at dump; sticky no invent empty dump past Type-nil shell
	ClearErrorSess(testAmbientSession)
	if (&Variable{Name: "g_broken"}).OutputValueDumpSess(testAmbientSession, "c ", 0, nil) != "" {
		t.Fatal("Type-nil OutputValueDump must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil OutputValueDump must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if outputValueDumpArraySess(testAmbientSession, &Variable{Name: "g_a", IsArray: true, ArraySizes: []int{2}}, "c ", 0, nil) != "" {
		t.Fatal("Type-nil outputValueDumpArray must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil outputValueDumpArray must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputValueDumpArrayPrintfDirectiveResidualSticky(t *testing.T) {
	// Aggregate field Type with nested Type-nil: PrintfDirective stickies residual "".
	// Field is simple? No — IsAggregate true → soft-skip non-simple without residual.
	// Soft invent residual: Type-nil field on union dump via IsFieldReadable already sticky.
	// Here: ambient residual ERROR before directive soft invents soft-continue later field.
	// Fair: residual sticky whole dump fail closed (empty dir + residual → "").
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	st := &Type{isStruct: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	f0 := &Variable{Name: "g_s.f0", Type: GetIntType()}
	f1 := &Variable{Name: "g_s.f1", Type: GetIntType()}
	av := &ArrayVariable{
		Variable: Variable{
			Name: "g_s", Type: st, IsArray: true, ArraySizes: []int{1},
			FieldVars: []*Variable{f0, f1},
			Qfer:      NewCVQualifiers([]bool{false}, []bool{false}),
		},
		Sizes: []int{1},
	}
	av.AsArray = av
	// Force residual from PrintfDirective path: nil Type field already sticky earlier.
	// Use Type-nil field Variable which stickies before PrintfDirective.
	f0.Type = nil
	if av.Variable.OutputValueDumpSess(testAmbientSession, "c ", 1, nil) != "" {
		t.Fatal("Type-nil field must fail closed array dump, not invent later field")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil field array dump must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
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
	g := NewProgramGenerator(NewSession(opts))
	g.GenerateAllTypes()
	// seed simple global
	v := CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntType(), false, false)
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
	g := NewProgramGenerator(NewSession(opts))
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
