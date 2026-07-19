package csmith

import (
	"strings"
	"testing"
)

func TestGetTypeFromString(t *testing.T) {
	if GetTypeFromString("Int") != GetIntType() {
		t.Fatal("Int")
	}
	if GetTypeFromString("Void").Simple() != EVoid {
		t.Fatal("Void")
	}
	if GetTypeFromString("ULonglong").Simple() != EULongLong {
		t.Fatal("ULonglong")
	}
	if GetTypeFromString("Nope") != nil {
		t.Fatal("unknown")
	}
	if GetIntType().TypeNameString() != "Int" {
		t.Fatal(GetIntType().TypeNameString())
	}
}

func TestSignedOverflowPossible(t *testing.T) {
	SetPlatformSizes(4, 8)
	defer SetPlatformSizes(4, 8)
	// char size 1 < int 4
	if GetSimpleType(EChar).SignedOverflowPossible(4) {
		t.Fatal("char")
	}
	if !GetIntType().SignedOverflowPossible(4) {
		t.Fatal("int")
	}
	if GetSimpleType(EUInt).SignedOverflowPossible(4) {
		t.Fatal("unsigned")
	}
	// no invent platform int_size when arg is 0
	if GetIntType().SignedOverflowPossible(0) {
		t.Fatal("intSize 0 must fail closed false")
	}
}

func TestSizeInBytesNoInventUnknownSimple(t *testing.T) {
	// unknown simple — no soft invent platform int size
	if n := (&Type{simple: ESimpleType(99)}).SizeInBytes(); n != 0 {
		t.Fatal("unknown simple SizeInBytes invent", n)
	}
}

func TestPrintfDirective(t *testing.T) {
	// pin platform so int is 4-byte (Generate may leave host int size)
	SetPlatformSizes(4, 8)
	defer SetPlatformSizes(4, 8)
	if GetIntType().PrintfDirective() != "%d" {
		t.Fatal(GetIntType().PrintfDirective())
	}
	if GetSimpleType(EUInt).PrintfDirective() != "%u" {
		t.Fatal("uint")
	}
	if GetSimpleType(ELongLong).PrintfDirective() != "%lld" {
		t.Fatal("ll")
	}
	if PointerTo(GetIntType()).PrintfDirective() != "0x%0x" {
		t.Fatal("ptr")
	}
	st := &Type{isStruct: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType()},
		{Name: "f1", Type: GetSimpleType(EUInt)},
	}}
	pd := st.PrintfDirective()
	if !strings.Contains(pd, "%d") || !strings.Contains(pd, "%u") {
		t.Fatal(pd)
	}
}

func TestSizeofString(t *testing.T) {
	if GetIntType().SizeofString() != "sizeof(int)" {
		t.Fatal(GetIntType().SizeofString())
	}
	// Type* always live; no invent sizeof(void)/sizeof()
	if s := (*Type)(nil).SizeofString(); s != "" {
		t.Fatal("nil sizeof invent", s)
	}
	if s := (&Type{isStruct: true}).SizeofString(); s != "" {
		t.Fatal("unnamed struct sizeof invent", s)
	}
}

func TestPointerToNoInventIntStar(t *testing.T) {
	// Type.cpp find_pointer_type — no soft invent int* for nil pointee
	if p := PointerTo(nil); p != nil {
		t.Fatal("PointerTo(nil) must fail closed nil, got", p.CName())
	}
}

func TestPrintfDirectiveNoInventFieldHoles(t *testing.T) {
	// Type.cpp fields[i]->printf_directive always live; no invent empty holes
	st := &Type{
		isStruct:   true,
		StructName: "S0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
			{Name: "f1", Type: nil, BitWidth: -1},
		},
	}
	if s := st.PrintfDirective(); s != "" {
		t.Fatal("nil field type must fail closed", s)
	}
}

func TestHasAggregateAndLongLongField(t *testing.T) {
	fields := []StructField{
		{Name: "f0", Type: GetIntType()},
		{Name: "f1", Type: GetSimpleType(ELongLong)},
	}
	if HasAggregateField(fields) {
		t.Fatal("no aggregate")
	}
	if !HasLongLongField(fields) {
		t.Fatal("ll")
	}
	fields = append(fields, StructField{Name: "f2", Type: &Type{isStruct: true}})
	if !HasAggregateField(fields) {
		t.Fatal("agg")
	}
}

func TestIsUnnamedPadding(t *testing.T) {
	st := &Type{isStruct: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: 0},
		{Name: "f1", Type: GetIntType(), BitWidth: 3},
	}}
	if !st.IsUnnamedPadding(0) {
		t.Fatal("pad")
	}
	if st.IsUnnamedPadding(1) {
		t.Fatal("named bitfield")
	}
}

func TestGetAllOKStructUnionTypesNilHole(t *testing.T) {
	env := &TypeEnv{AllTypes: []*Type{GetIntType(), nil}}
	if typesComplete(env.GetAllOKStructUnionTypes(false, false, false, true)) {
		t.Fatal("nil type hole must fail closed incomplete, not invent empty complete")
	}
	// complete empty filter (no structs) is complete empty non-nil
	env2 := &TypeEnv{AllTypes: []*Type{GetIntType()}}
	ok := env2.GetAllOKStructUnionTypes(false, false, false, true)
	if !typesComplete(ok) || len(ok) != 0 {
		t.Fatal("no structs must be complete empty", ok)
	}
}

func TestGetSimpleTypeOOBNoInventInt(t *testing.T) {
	// Type.cpp get_simple_type assert path — no invent eInt for invalid enum
	if GetSimpleType(ESimpleType(-1)) != nil {
		t.Fatal("negative eSimpleType must fail closed nil")
	}
	if GetSimpleType(ESimpleType(MaxSimpleTypes)) != nil {
		t.Fatal("OOB eSimpleType must fail closed nil, not invent eInt")
	}
	if GetSimpleType(ESimpleType(MaxSimpleTypes+99)) != nil {
		t.Fatal("far OOB must fail closed nil")
	}
	if GetSimpleType(EInt) != GetIntType() {
		t.Fatal("valid eInt must still resolve")
	}
}

func TestFindTypeNilHole(t *testing.T) {
	// Type* always live on AllTypes; no invent soft-skip hole then match later
	ClearError()
	intT := GetIntType()
	env := &TypeEnv{AllTypes: []*Type{nil, intT}}
	if env.FindType(intT) != nil {
		t.Fatal("nil AllTypes hole must fail closed FindType (not soft-skip to match)")
	}
	if !HasError() {
		t.Fatal("nil AllTypes hole must SetError sticky")
	}
	ClearError()
	// complete pool still finds
	envOK := &TypeEnv{AllTypes: []*Type{GetSimpleType(EShort), intT}}
	if envOK.FindType(intT) != intT {
		t.Fatal("complete pool must find type")
	}
}

func TestGetAllOKStructUnionTypes(t *testing.T) {
	env := &TypeEnv{}
	st := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: GetIntType()},
	}}
	ut := &Type{isUnion: true, StructName: "U0", Fields: []StructField{
		{Name: "f0", Type: GetIntType()},
	}}
	env.AllTypes = []*Type{GetIntType(), st, ut}
	structs := env.GetAllOKStructUnionTypes(false, false, true, true)
	if len(structs) != 1 || structs[0] != st {
		t.Fatal(structs)
	}
	unions := env.GetAllOKStructUnionTypes(false, false, true, false)
	if len(unions) != 1 || unions[0] != ut {
		t.Fatal(unions)
	}
	if env.FindType(st) != st {
		t.Fatal("find")
	}
	if env.FindType(GetSimpleType(EChar)) != nil {
		t.Fatal("not in AllTypes")
	}
}

func TestChooseRandomStructFromType(t *testing.T) {
	env := &TypeEnv{}
	st := &Type{isStruct: true, StructName: "S0"}
	env.AllTypes = []*Type{st}
	if env.ChooseRandomStructFromType(NewRng(1), st, false) != st {
		t.Fatal("same")
	}
	got := ChooseRandomStructUnionType(NewRng(2), []*Type{st})
	if got != st {
		t.Fatal(got)
	}
}

func TestIfStructAssignOps(t *testing.T) {
	opts := Defaults()
	if IfStructWillHaveAssignOps(NewRng(1), opts, NewProbabilities(opts)) {
		t.Fatal("C mode false")
	}
	opts.LangCPP = true
	// may or may not flip — just exercise
	_ = IfUnionWillHaveAssignOps(NewRng(3), opts, NewProbabilities(opts))
	// nil probs → 0% (no invent default 50)
	for seed := uint64(1); seed < 30; seed++ {
		if IfStructWillHaveAssignOps(NewRng(seed), opts, nil) {
			t.Fatal("nil probs must not invent assign-ops true at 50%")
		}
	}
}

func TestMoreTypesProbabilityNilProbs(t *testing.T) {
	// below threshold still true; above threshold nil probs → 0% not invent 50
	if !MoreTypesProbability(NewRng(1), nil, 5) {
		t.Fatal("count<10 always true")
	}
	for seed := uint64(1); seed < 40; seed++ {
		if MoreTypesProbability(NewRng(seed), nil, 20) {
			t.Fatal("nil probs past threshold must not invent 50% true")
		}
	}
}
