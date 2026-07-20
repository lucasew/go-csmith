package csmith

import (
	"strings"
	"testing"
)

func TestGetTypeFromString(t *testing.T) {
	ClearError()
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
	if !HasError() {
		t.Fatal("unknown type string must SetError sticky")
	}
	ClearError()
	if GetIntType().TypeNameString() != "Int" {
		t.Fatal(GetIntType().TypeNameString())
	}
	// unknown simple TypeNameString — sticky no invent empty token
	if s := (&Type{simple: ESimpleType(99)}).TypeNameString(); s != "" {
		t.Fatal("unknown simple TypeNameString invent", s)
	}
	if !HasError() {
		t.Fatal("unknown simple TypeNameString must SetError sticky")
	}
	ClearError()
	// empty struct/union tag sticky (parity with CName; no invent empty type-name)
	if s := (&Type{isStruct: true, StructName: ""}).TypeNameString(); s != "" {
		t.Fatal("empty struct tag TypeNameString invent", s)
	}
	if !HasError() {
		t.Fatal("empty struct tag TypeNameString must SetError sticky")
	}
	ClearError()
	if s := (&Type{isUnion: true, StructName: ""}).TypeNameString(); s != "" {
		t.Fatal("empty union tag TypeNameString invent", s)
	}
	if !HasError() {
		t.Fatal("empty union tag TypeNameString must SetError sticky")
	}
	ClearError()
	if (&Type{isStruct: true, StructName: "S0"}).TypeNameString() != "S0" {
		t.Fatal("struct tag")
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
	// Type* always live; sticky true (no invent overflow-free soft-skip)
	ClearError()
	if !(*Type)(nil).SignedOverflowPossible(4) {
		t.Fatal("nil SignedOverflowPossible must fail closed true")
	}
	if !HasError() {
		t.Fatal("nil SignedOverflowPossible must SetError sticky")
	}
	ClearError()
}

func TestTypeNilHardQuerySticky(t *testing.T) {
	// Type* always live at hard query/emit; sticky no invent soft success past hole
	ClearError()
	if (*Type)(nil).Simple() != EVoid {
		t.Fatal("nil Simple must fail closed EVoid")
	}
	if !HasError() {
		t.Fatal("nil Simple must SetError sticky")
	}
	ClearError()
	if (*Type)(nil).PtrType() != nil {
		t.Fatal("nil PtrType must fail closed nil")
	}
	if !HasError() {
		t.Fatal("nil PtrType must SetError sticky")
	}
	ClearError()
	if (*Type)(nil).ToUnsigned() != nil {
		t.Fatal("nil ToUnsigned must fail closed nil")
	}
	if !HasError() {
		t.Fatal("nil ToUnsigned must SetError sticky")
	}
	ClearError()
	if !(*Type)(nil).ContainPointerField() {
		t.Fatal("nil ContainPointerField must fail closed true")
	}
	if !HasError() {
		t.Fatal("nil ContainPointerField must SetError sticky")
	}
	ClearError()
	if !(*Type)(nil).HasBitfields() {
		t.Fatal("nil HasBitfields must fail closed true")
	}
	if !HasError() {
		t.Fatal("nil HasBitfields must SetError sticky")
	}
	ClearError()
	if !(*Type)(nil).HasPadding() {
		t.Fatal("nil HasPadding must fail closed true")
	}
	if !HasError() {
		t.Fatal("nil HasPadding must SetError sticky")
	}
	ClearError()
	if (*Type)(nil).IsBitfieldIndex(0) {
		t.Fatal("nil IsBitfieldIndex must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil IsBitfieldIndex must SetError sticky")
	}
	ClearError()
	if (*Type)(nil).IsUnamedPadding(0) {
		t.Fatal("nil IsUnamedPadding must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil IsUnamedPadding must SetError sticky")
	}
	ClearError()
	if (*Type)(nil).IsUnnamedPadding(0) {
		t.Fatal("nil IsUnnamedPadding must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil IsUnnamedPadding must SetError sticky")
	}
	ClearError()
	if s := (*Type)(nil).TypeNameString(); s != "" {
		t.Fatal("nil TypeNameString invent", s)
	}
	if !HasError() {
		t.Fatal("nil TypeNameString must SetError sticky")
	}
	ClearError()
	if s := (*Type)(nil).PrintfDirective(); s != "" {
		t.Fatal("nil PrintfDirective invent", s)
	}
	if !HasError() {
		t.Fatal("nil PrintfDirective must SetError sticky")
	}
	ClearError()
	if (*Type)(nil).HasIntField() {
		t.Fatal("nil HasIntField must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil HasIntField must SetError sticky")
	}
	ClearError()
	if !(*Type)(nil).IsFullBitfieldsStruct() {
		t.Fatal("nil IsFullBitfieldsStruct must fail closed true")
	}
	if !HasError() {
		t.Fatal("nil IsFullBitfieldsStruct must SetError sticky")
	}
	ClearError()
	if !(*Type)(nil).IsSigned() {
		t.Fatal("nil IsSigned must fail closed true")
	}
	if !HasError() {
		t.Fatal("nil IsSigned must SetError sticky")
	}
	ClearError()
	if (*Type)(nil).IsFloat() {
		t.Fatal("nil IsFloat must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil IsFloat must SetError sticky")
	}
	ClearError()
	if (*Type)(nil).IsSignedChar() {
		t.Fatal("nil IsSignedChar must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil IsSignedChar must SetError sticky")
	}
	ClearError()
	if (*Type)(nil).Match(GetIntType(), MatchExact) {
		t.Fatal("nil Match must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil Match must SetError sticky")
	}
	ClearError()
	if (*Type)(nil).IsPromotable(GetIntType()) {
		t.Fatal("nil IsPromotable must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil IsPromotable must SetError sticky")
	}
	ClearError()
	if (*Type)(nil).IsEquivalent(GetIntType()) {
		t.Fatal("nil IsEquivalent must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil IsEquivalent must SetError sticky")
	}
	ClearError()
	if (*Type)(nil).IsDereferencedFrom(PointerTo(GetIntType())) {
		t.Fatal("nil IsDereferencedFrom must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil IsDereferencedFrom must SetError sticky")
	}
	ClearError()
	if (*Type)(nil).IsDerivable(GetIntType()) {
		t.Fatal("nil IsDerivable must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil IsDerivable must SetError sticky")
	}
	ClearError()
}

func TestSizeInBytesNoInventUnknownSimple(t *testing.T) {
	// unknown simple — sticky no soft invent platform int size
	ClearError()
	if n := (&Type{simple: ESimpleType(99)}).SizeInBytes(); n != 0 {
		t.Fatal("unknown simple SizeInBytes invent", n)
	}
	if !HasError() {
		t.Fatal("unknown simple SizeInBytes must SetError sticky")
	}
	ClearError()
}

func TestSizeInBytesNilSticky(t *testing.T) {
	// Type* always live at SizeInBytes; sticky no invent zero-size soft-skip
	ClearError()
	if n := (*Type)(nil).SizeInBytes(); n != 0 {
		t.Fatal("nil SizeInBytes invent", n)
	}
	if !HasError() {
		t.Fatal("nil SizeInBytes must SetError sticky")
	}
	ClearError()
}

func TestIndirectLevelBaseTypeNilSticky(t *testing.T) {
	// Type* always live; sticky no invent level-0 / missing base soft-skip past hole
	ClearError()
	if (*Type)(nil).IndirectLevel() != 0 {
		t.Fatal("nil IndirectLevel must fail closed 0")
	}
	if !HasError() {
		t.Fatal("nil IndirectLevel must SetError sticky")
	}
	ClearError()
	if (*Type)(nil).BaseType() != nil {
		t.Fatal("nil BaseType must fail closed nil")
	}
	if !HasError() {
		t.Fatal("nil BaseType must SetError sticky")
	}
	ClearError()
	// complete pointer still peels
	p := PointerTo(GetIntType())
	if p.IndirectLevel() != 1 {
		t.Fatal(p.IndirectLevel())
	}
	if p.BaseType() != GetIntType() {
		t.Fatal(p.BaseType())
	}
	if HasError() {
		t.Fatal("complete IndirectLevel/BaseType must not sticky")
	}
	ClearError()
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
	ClearError()
	if GetIntType().SizeofString() != "sizeof(int)" {
		t.Fatal(GetIntType().SizeofString())
	}
	// Type* always live; sticky no invent sizeof(void)/sizeof()
	ClearError()
	if s := (*Type)(nil).SizeofString(); s != "" {
		t.Fatal("nil sizeof invent", s)
	}
	if !HasError() {
		t.Fatal("nil SizeofString must SetError sticky")
	}
	ClearError()
	if s := (&Type{isStruct: true}).SizeofString(); s != "" {
		t.Fatal("unnamed struct sizeof invent", s)
	}
	if !HasError() {
		t.Fatal("unnamed struct sizeof must SetError sticky")
	}
	ClearError()
}

func TestPointerToNoInventIntStar(t *testing.T) {
	// Type.cpp find_pointer_type sticky — no soft invent int* for nil pointee
	ClearError()
	if p := PointerTo(nil); p != nil {
		t.Fatal("PointerTo(nil) must fail closed nil, got", p.CName())
	}
	if !HasError() {
		t.Fatal("PointerTo(nil) must SetError sticky")
	}
	ClearError()
}

func TestPrintfDirectiveNoInventFieldHoles(t *testing.T) {
	// Type.cpp fields[i]->printf_directive always live; sticky no invent empty holes
	ClearError()
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
	if !HasError() {
		t.Fatal("nil field type PrintfDirective must SetError sticky")
	}
	ClearError()
	// nested residual soft invent was soft-continue later fields invent partial "{%d, {%d}}"
	// nested aggregate with Type-nil field stickies nested PrintfDirective
	nested := &Type{
		isStruct:   true,
		StructName: "Nest",
		Fields: []StructField{
			{Name: "x", Type: GetIntType(), BitWidth: -1},
			{Name: "y", Type: nil, BitWidth: -1},
		},
	}
	st2 := &Type{
		isStruct:   true,
		StructName: "S1",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
			{Name: "f1", Type: nested, BitWidth: -1},
			{Name: "f2", Type: GetIntType(), BitWidth: -1},
		},
	}
	if s := st2.PrintfDirective(); s != "" {
		t.Fatal("nested residual must fail closed whole PrintfDirective, not invent partial", s)
	}
	if !HasError() {
		t.Fatal("nested residual PrintfDirective must SetError sticky")
	}
	ClearError()
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
	// nil field Type sticky has-aggregate (no invent none / soft re-pick past hole)
	ClearError()
	if !HasAggregateField([]StructField{{Name: "f0", Type: nil}}) {
		t.Fatal("nil field Type must fail closed true")
	}
	if !HasError() {
		t.Fatal("nil field Type HasAggregateField must SetError sticky")
	}
	ClearError()
}

func TestIsUnnamedPadding(t *testing.T) {
	ClearError()
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
	// Type.cpp assert OOB sticky
	if st.IsBitfieldIndex(99) {
		t.Fatal("OOB IsBitfieldIndex invent")
	}
	if !HasError() {
		t.Fatal("OOB IsBitfieldIndex must SetError sticky")
	}
	ClearError()
	if st.IsUnamedPadding(-1) {
		t.Fatal("neg IsUnamedPadding invent")
	}
	if !HasError() {
		t.Fatal("OOB IsUnamedPadding must SetError sticky")
	}
	ClearError()
}

func TestGetAllOKStructUnionTypesNilHole(t *testing.T) {
	ClearError()
	env := &TypeEnv{AllTypes: []*Type{GetIntType(), nil}}
	if typesComplete(env.GetAllOKStructUnionTypes(false, false, false, true)) {
		t.Fatal("nil type hole must fail closed incomplete, not invent empty complete")
	}
	if !HasError() {
		t.Fatal("nil type hole must SetError sticky")
	}
	ClearError()
	// complete empty filter (no structs) is complete empty non-nil
	env2 := &TypeEnv{AllTypes: []*Type{GetIntType()}}
	ok := env2.GetAllOKStructUnionTypes(false, false, false, true)
	if !typesComplete(ok) || len(ok) != 0 {
		t.Fatal("no structs must be complete empty", ok)
	}
}

func TestGetSimpleTypeOOBNoInventInt(t *testing.T) {
	// Type.cpp get_simple_type assert path — sticky, no invent eInt for invalid enum
	ClearError()
	if GetSimpleType(ESimpleType(-1)) != nil {
		t.Fatal("negative eSimpleType must fail closed nil")
	}
	if !HasError() {
		t.Fatal("negative eSimpleType must SetError sticky")
	}
	ClearError()
	if GetSimpleType(ESimpleType(MaxSimpleTypes)) != nil {
		t.Fatal("OOB eSimpleType must fail closed nil, not invent eInt")
	}
	if !HasError() {
		t.Fatal("OOB eSimpleType must SetError sticky")
	}
	ClearError()
	if GetSimpleType(ESimpleType(MaxSimpleTypes+99)) != nil {
		t.Fatal("far OOB must fail closed nil")
	}
	if !HasError() {
		t.Fatal("far OOB must SetError sticky")
	}
	ClearError()
	if GetSimpleType(EInt) != GetIntType() {
		t.Fatal("valid eInt must still resolve")
	}
	if HasError() {
		t.Fatal("valid eInt must not leave sticky error")
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

func TestGetAllOKStructUnionTypesFilterResidualSticky(t *testing.T) {
	// Type-nil field IsConstStructUnion stickies residual; soft invent was soft-skip then pick good.
	// Fair: sticky IncompleteTypes fail closed whole filter.
	ClearError()
	broken := &Type{isStruct: true, StructName: "Sbad", Fields: []StructField{
		{Name: "f0", Type: nil},
	}}
	good := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: GetIntType()},
	}}
	env := &TypeEnv{AllTypes: []*Type{broken, good}}
	if typesComplete(env.GetAllOKStructUnionTypes(true, false, false, true)) {
		t.Fatal("IsConstStructUnion residual must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("IsConstStructUnion residual must SetError sticky")
	}
	ClearError()
	// needIntField HasIntField residual
	env2 := &TypeEnv{AllTypes: []*Type{broken, good}}
	if typesComplete(env2.GetAllOKStructUnionTypes(false, false, true, true)) {
		t.Fatal("HasIntField residual must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("HasIntField residual must SetError sticky")
	}
	ClearError()
	// okStructUnionLTypes noVolatile residual
	env3 := &TypeEnv{StructTypes: []*Type{broken, good}}
	if typesComplete(okStructUnionLTypes(env3, true, true, false)) {
		t.Fatal("IsVolatileStructUnion residual must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("IsVolatileStructUnion residual must SetError sticky")
	}
	ClearError()
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
	// RNG always live under C++; sticky false (no invent no-assign-ops soft-skip)
	ClearError()
	if IfStructWillHaveAssignOps(nil, opts, NewProbabilities(opts)) {
		t.Fatal("nil RNG IfStructWillHaveAssignOps must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil RNG IfStructWillHaveAssignOps must SetError sticky")
	}
	ClearError()
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
