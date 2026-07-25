package csmith

import (
	"strings"
	"testing"
)

func TestGetTypeFromString(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if GetTypeFromStringSess(testAmbientSession, "Int") != GetIntTypeSess(testAmbientSession) {
		t.Fatal("Int")
	}
	if GetTypeFromStringSess(testAmbientSession, "Void").SimpleSess(testAmbientSession) != EVoid {
		t.Fatal("Void")
	}
	if GetTypeFromStringSess(testAmbientSession, "ULonglong").SimpleSess(testAmbientSession) != EULongLong {
		t.Fatal("ULonglong")
	}
	if GetTypeFromStringSess(testAmbientSession, "Nope") != nil {
		t.Fatal("unknown")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("unknown type string must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if GetIntTypeSess(testAmbientSession).TypeNameStringSess(testAmbientSession) != "Int" {
		t.Fatal(GetIntTypeSess(testAmbientSession).TypeNameStringSess(testAmbientSession))
	}
	// unknown simple TypeNameString — sticky no invent empty token
	if s := (&Type{simple: ESimpleType(99)}).TypeNameStringSess(testAmbientSession); s != "" {
		t.Fatal("unknown simple TypeNameString invent", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("unknown simple TypeNameString must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// empty struct/union tag sticky (parity with CName; no invent empty type-name)
	if s := (&Type{isStruct: true, StructName: ""}).TypeNameStringSess(testAmbientSession); s != "" {
		t.Fatal("empty struct tag TypeNameString invent", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty struct tag TypeNameString must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if s := (&Type{isUnion: true, StructName: ""}).TypeNameStringSess(testAmbientSession); s != "" {
		t.Fatal("empty union tag TypeNameString invent", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty union tag TypeNameString must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&Type{isStruct: true, StructName: "S0"}).TypeNameStringSess(testAmbientSession) != "S0" {
		t.Fatal("struct tag")
	}
}

func TestSignedOverflowPossible(t *testing.T) {
	SetPlatformSizesSess(testAmbientSession, 4, 8)
	defer SetPlatformSizesSess(testAmbientSession, 4, 8)
	// char size 1 < int 4
	if GetSimpleTypeSess(testAmbientSession, EChar).SignedOverflowPossibleSess(testAmbientSession, 4) {
		t.Fatal("char")
	}
	if !GetIntTypeSess(testAmbientSession).SignedOverflowPossibleSess(testAmbientSession, 4) {
		t.Fatal("int")
	}
	if GetSimpleTypeSess(testAmbientSession, EUInt).SignedOverflowPossibleSess(testAmbientSession, 4) {
		t.Fatal("unsigned")
	}
	// no invent platform int_size when arg is 0
	if GetIntTypeSess(testAmbientSession).SignedOverflowPossibleSess(testAmbientSession, 0) {
		t.Fatal("intSize 0 must fail closed false")
	}
	// Type* always live; sticky true (no invent overflow-free soft-skip)
	ClearErrorSess(testAmbientSession)
	if !(*Type)(nil).SignedOverflowPossibleSess(testAmbientSession, 4) {
		t.Fatal("nil SignedOverflowPossible must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil SignedOverflowPossible must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestTypeNilHardQuerySticky(t *testing.T) {
	// Type* always live at hard query/emit; sticky no invent soft success past hole
	ClearErrorSess(testAmbientSession)
	if (*Type)(nil).SimpleSess(testAmbientSession) != EVoid {
		t.Fatal("nil Simple must fail closed EVoid")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Simple must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Type)(nil).PtrTypeSess(testAmbientSession) != nil {
		t.Fatal("nil PtrType must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil PtrType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Type)(nil).ToUnsignedSess(testAmbientSession) != nil {
		t.Fatal("nil ToUnsigned must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil ToUnsigned must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !(*Type)(nil).ContainPointerFieldSess(testAmbientSession) {
		t.Fatal("nil ContainPointerField must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil ContainPointerField must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !(*Type)(nil).HasBitfieldsSess(testAmbientSession) {
		t.Fatal("nil HasBitfields must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil HasBitfields must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !(*Type)(nil).HasPaddingSess(testAmbientSession) {
		t.Fatal("nil HasPadding must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil HasPadding must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Type)(nil).IsBitfieldIndexSess(testAmbientSession, 0) {
		t.Fatal("nil IsBitfieldIndex must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsBitfieldIndex must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Type)(nil).IsUnamedPaddingSess(testAmbientSession, 0) {
		t.Fatal("nil IsUnamedPadding must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsUnamedPadding must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Type)(nil).IsUnnamedPaddingSess(testAmbientSession, 0) {
		t.Fatal("nil IsUnnamedPadding must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsUnnamedPadding must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if s := (*Type)(nil).TypeNameStringSess(testAmbientSession); s != "" {
		t.Fatal("nil TypeNameString invent", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil TypeNameString must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if s := (*Type)(nil).PrintfDirectiveSess(testAmbientSession); s != "" {
		t.Fatal("nil PrintfDirective invent", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil PrintfDirective must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Type)(nil).HasIntFieldSess(testAmbientSession) {
		t.Fatal("nil HasIntField must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil HasIntField must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !(*Type)(nil).IsFullBitfieldsStructSess(testAmbientSession) {
		t.Fatal("nil IsFullBitfieldsStruct must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsFullBitfieldsStruct must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !(*Type)(nil).IsSignedSess(testAmbientSession) {
		t.Fatal("nil IsSigned must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsSigned must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Type)(nil).IsFloatSess(testAmbientSession) {
		t.Fatal("nil IsFloat must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsFloat must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Type)(nil).IsSignedCharSess(testAmbientSession) {
		t.Fatal("nil IsSignedChar must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsSignedChar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Type)(nil).MatchSess(testAmbientSession, GetIntTypeSess(testAmbientSession), MatchExact) {
		t.Fatal("nil Match must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Match must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Type)(nil).IsPromotableSess(testAmbientSession, GetIntTypeSess(testAmbientSession)) {
		t.Fatal("nil IsPromotable must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsPromotable must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Type)(nil).IsEquivalentSess(testAmbientSession, GetIntTypeSess(testAmbientSession)) {
		t.Fatal("nil IsEquivalent must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsEquivalent must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Type)(nil).IsDereferencedFromSess(testAmbientSession, PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))) {
		t.Fatal("nil IsDereferencedFrom must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsDereferencedFrom must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Type)(nil).IsDerivableSess(testAmbientSession, GetIntTypeSess(testAmbientSession)) {
		t.Fatal("nil IsDerivable must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsDerivable must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSizeInBytesNoInventUnknownSimple(t *testing.T) {
	// unknown simple — sticky no soft invent platform int size
	ClearErrorSess(testAmbientSession)
	if n := (&Type{simple: ESimpleType(99)}).SizeInBytesSess(testAmbientSession); n != 0 {
		t.Fatal("unknown simple SizeInBytes invent", n)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("unknown simple SizeInBytes must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSizeInBytesNilSticky(t *testing.T) {
	// Type* always live at SizeInBytes; sticky no invent zero-size soft-skip
	ClearErrorSess(testAmbientSession)
	if n := (*Type)(nil).SizeInBytesSess(testAmbientSession); n != 0 {
		t.Fatal("nil SizeInBytes invent", n)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil SizeInBytes must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIndirectLevelBaseTypeNilSticky(t *testing.T) {
	// Type* always live; sticky no invent level-0 / missing base soft-skip past hole
	ClearErrorSess(testAmbientSession)
	if (*Type)(nil).IndirectLevelSess(testAmbientSession) != 0 {
		t.Fatal("nil IndirectLevel must fail closed 0")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IndirectLevel must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Type)(nil).BaseTypeSess(testAmbientSession) != nil {
		t.Fatal("nil BaseType must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil BaseType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete pointer still peels
	p := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	if p.IndirectLevelSess(testAmbientSession) != 1 {
		t.Fatal(p.IndirectLevelSess(testAmbientSession))
	}
	if p.BaseTypeSess(testAmbientSession) != GetIntTypeSess(testAmbientSession) {
		t.Fatal(p.BaseTypeSess(testAmbientSession))
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete IndirectLevel/BaseType must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestPrintfDirective(t *testing.T) {
	// pin platform so int is 4-byte (Generate may leave host int size)
	SetPlatformSizesSess(testAmbientSession, 4, 8)
	defer SetPlatformSizesSess(testAmbientSession, 4, 8)
	if GetIntTypeSess(testAmbientSession).PrintfDirectiveSess(testAmbientSession) != "%d" {
		t.Fatal(GetIntTypeSess(testAmbientSession).PrintfDirectiveSess(testAmbientSession))
	}
	if GetSimpleTypeSess(testAmbientSession, EUInt).PrintfDirectiveSess(testAmbientSession) != "%u" {
		t.Fatal("uint")
	}
	if GetSimpleTypeSess(testAmbientSession, ELongLong).PrintfDirectiveSess(testAmbientSession) != "%lld" {
		t.Fatal("ll")
	}
	if PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)).PrintfDirectiveSess(testAmbientSession) != "0x%0x" {
		t.Fatal("ptr")
	}
	st := &Type{isStruct: true, Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession)},
		{Name: "f1", Type: GetSimpleTypeSess(testAmbientSession, EUInt)},
	}}
	pd := st.PrintfDirectiveSess(testAmbientSession)
	if !strings.Contains(pd, "%d") || !strings.Contains(pd, "%u") {
		t.Fatal(pd)
	}
}

func TestSizeofString(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if GetIntTypeSess(testAmbientSession).SizeofStringSess(testAmbientSession) != "sizeof(int32_t)" {
		t.Fatal(GetIntTypeSess(testAmbientSession).SizeofStringSess(testAmbientSession))
	}
	// Type* always live; sticky no invent sizeof(void)/sizeof()
	ClearErrorSess(testAmbientSession)
	if s := (*Type)(nil).SizeofStringSess(testAmbientSession); s != "" {
		t.Fatal("nil sizeof invent", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil SizeofString must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if s := (&Type{isStruct: true}).SizeofStringSess(testAmbientSession); s != "" {
		t.Fatal("unnamed struct sizeof invent", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("unnamed struct sizeof must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestPointerToNoInventIntStar(t *testing.T) {
	// Type.cpp find_pointer_type sticky — no soft invent int* for nil pointee
	ClearErrorSess(testAmbientSession)
	if p := PointerToSess(testAmbientSession, nil); p != nil {
		t.Fatal("PointerToSess(testAmbientSession, nil) must fail closed nil, got", p.CNameSess(testAmbientSession))
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("PointerToSess(testAmbientSession, nil) must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestPrintfDirectiveNoInventFieldHoles(t *testing.T) {
	// Type.cpp fields[i]->printf_directive always live; sticky no invent empty holes
	ClearErrorSess(testAmbientSession)
	st := &Type{
		isStruct:   true,
		StructName: "S0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
			{Name: "f1", Type: nil, BitWidth: -1},
		},
	}
	if s := st.PrintfDirectiveSess(testAmbientSession); s != "" {
		t.Fatal("nil field type must fail closed", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil field type PrintfDirective must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nested residual soft invent was soft-continue later fields invent partial "{%d, {%d}}"
	// nested aggregate with Type-nil field stickies nested PrintfDirective
	nested := &Type{
		isStruct:   true,
		StructName: "Nest",
		Fields: []StructField{
			{Name: "x", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
			{Name: "y", Type: nil, BitWidth: -1},
		},
	}
	st2 := &Type{
		isStruct:   true,
		StructName: "S1",
		Fields: []StructField{
			{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
			{Name: "f1", Type: nested, BitWidth: -1},
			{Name: "f2", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		},
	}
	if s := st2.PrintfDirectiveSess(testAmbientSession); s != "" {
		t.Fatal("nested residual must fail closed whole PrintfDirective, not invent partial", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nested residual PrintfDirective must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestHasAggregateAndLongLongField(t *testing.T) {
	fields := []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession)},
		{Name: "f1", Type: GetSimpleTypeSess(testAmbientSession, ELongLong)},
	}
	if HasAggregateFieldSess(testAmbientSession, fields) {
		t.Fatal("no aggregate")
	}
	if !HasLongLongFieldSess(testAmbientSession, fields) {
		t.Fatal("ll")
	}
	fields = append(fields, StructField{Name: "f2", Type: &Type{isStruct: true}})
	if !HasAggregateFieldSess(testAmbientSession, fields) {
		t.Fatal("agg")
	}
	// nil field Type sticky has-aggregate (no invent none / soft re-pick past hole)
	ClearErrorSess(testAmbientSession)
	if !HasAggregateFieldSess(testAmbientSession, []StructField{{Name: "f0", Type: nil}}) {
		t.Fatal("nil field Type must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil field Type HasAggregateField must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsUnnamedPadding(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	st := &Type{isStruct: true, Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: 0},
		{Name: "f1", Type: GetIntTypeSess(testAmbientSession), BitWidth: 3},
	}}
	if !st.IsUnnamedPaddingSess(testAmbientSession, 0) {
		t.Fatal("pad")
	}
	if st.IsUnnamedPaddingSess(testAmbientSession, 1) {
		t.Fatal("named bitfield")
	}
	// Type.cpp assert OOB sticky
	if st.IsBitfieldIndexSess(testAmbientSession, 99) {
		t.Fatal("OOB IsBitfieldIndex invent")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("OOB IsBitfieldIndex must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if st.IsUnamedPaddingSess(testAmbientSession, -1) {
		t.Fatal("neg IsUnamedPadding invent")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("OOB IsUnamedPadding must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGetAllOKStructUnionTypesNilHole(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	env := &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetIntTypeSess(testAmbientSession), nil}}
	if typesComplete(env.GetAllOKStructUnionTypes(false, false, false, true)) {
		t.Fatal("nil type hole must fail closed incomplete, not invent empty complete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete empty filter (no structs) is complete empty non-nil
	env2 := &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetIntTypeSess(testAmbientSession)}}
	ok := env2.GetAllOKStructUnionTypes(false, false, false, true)
	if !typesComplete(ok) || len(ok) != 0 {
		t.Fatal("no structs must be complete empty", ok)
	}
}

func TestGetSimpleTypeOOBNoInventInt(t *testing.T) {
	// Type.cpp get_simple_type assert path — sticky, no invent eInt for invalid enum
	ClearErrorSess(testAmbientSession)
	if GetSimpleTypeSess(testAmbientSession, ESimpleType(-1)) != nil {
		t.Fatal("negative eSimpleType must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("negative eSimpleType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if GetSimpleTypeSess(testAmbientSession, ESimpleType(MaxSimpleTypes)) != nil {
		t.Fatal("OOB eSimpleType must fail closed nil, not invent eInt")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("OOB eSimpleType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if GetSimpleTypeSess(testAmbientSession, ESimpleType(MaxSimpleTypes+99)) != nil {
		t.Fatal("far OOB must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("far OOB must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if GetSimpleTypeSess(testAmbientSession, EInt) != GetIntTypeSess(testAmbientSession) {
		t.Fatal("valid eInt must still resolve")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("valid eInt must not leave sticky error")
	}
}

func TestFindTypeNilHole(t *testing.T) {
	// Type* always live on AllTypes; no invent soft-skip hole then match later
	ClearErrorSess(testAmbientSession)
	intT := GetIntTypeSess(testAmbientSession)
	env := &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{nil, intT}}
	if env.FindType(intT) != nil {
		t.Fatal("nil AllTypes hole must fail closed FindType (not soft-skip to match)")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil AllTypes hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete pool still finds
	envOK := &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetSimpleTypeSess(testAmbientSession, EShort), intT}}
	if envOK.FindType(intT) != intT {
		t.Fatal("complete pool must find type")
	}
}

func TestGetAllOKStructUnionTypesFilterResidualSticky(t *testing.T) {
	// Type-nil field IsConstStructUnion stickies residual; soft invent was soft-skip then pick good.
	// Fair: sticky IncompleteTypes fail closed whole filter.
	ClearErrorSess(testAmbientSession)
	broken := &Type{isStruct: true, StructName: "Sbad", Fields: []StructField{
		{Name: "f0", Type: nil},
	}}
	good := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession)},
	}}
	env := &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{broken, good}}
	if typesComplete(env.GetAllOKStructUnionTypes(true, false, false, true)) {
		t.Fatal("IsConstStructUnion residual must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsConstStructUnion residual must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// needIntField HasIntField residual
	env2 := &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{broken, good}}
	if typesComplete(env2.GetAllOKStructUnionTypes(false, false, true, true)) {
		t.Fatal("HasIntField residual must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("HasIntField residual must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// okStructUnionLTypes noVolatile residual
	env3 := &TypeEnv{Sess: testAmbientSession, StructTypes: []*Type{broken, good}}
	if typesComplete(okStructUnionLTypes(env3, true, true, false)) {
		t.Fatal("IsVolatileStructUnion residual must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsVolatileStructUnion residual must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGetAllOKStructUnionTypes(t *testing.T) {
	env := &TypeEnv{Sess: testAmbientSession}
	st := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession)},
	}}
	ut := &Type{isUnion: true, StructName: "U0", Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession)},
	}}
	env.AllTypes = []*Type{GetIntTypeSess(testAmbientSession), st, ut}
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
	if env.FindType(GetSimpleTypeSess(testAmbientSession, EChar)) != nil {
		t.Fatal("not in AllTypes")
	}
}

func TestChooseRandomStructFromType(t *testing.T) {
	env := &TypeEnv{Sess: testAmbientSession}
	st := &Type{isStruct: true, StructName: "S0"}
	env.AllTypes = []*Type{st}
	if env.ChooseRandomStructFromType(NewRngSess(testAmbientSession, 1), st, false) != st {
		t.Fatal("same")
	}
	got := ChooseRandomStructUnionType(NewRngSess(testAmbientSession, 2), []*Type{st})
	if got != st {
		t.Fatal(got)
	}
}

func TestIfStructAssignOps(t *testing.T) {
	opts := Defaults()
	if IfStructWillHaveAssignOpsSess(testAmbientSession, NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts)) {
		t.Fatal("C mode false")
	}
	opts.LangCPP = true
	// may or may not flip — just exercise
	_ = IfUnionWillHaveAssignOpsSess(testAmbientSession, NewRngSess(testAmbientSession, 3), opts, NewProbabilities(opts))
	// nil probs → 0% (no invent default 50)
	for seed := uint64(1); seed < 30; seed++ {
		if IfStructWillHaveAssignOpsSess(testAmbientSession, NewRngSess(testAmbientSession, seed), opts, nil) {
			t.Fatal("nil probs must not invent assign-ops true at 50%")
		}
	}
	// RNG always live under C++; sticky false (no invent no-assign-ops soft-skip)
	ClearErrorSess(testAmbientSession)
	if IfStructWillHaveAssignOpsSess(testAmbientSession, nil, opts, NewProbabilities(opts)) {
		t.Fatal("nil RNG IfStructWillHaveAssignOps must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG IfStructWillHaveAssignOps must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMoreTypesProbabilityNilProbs(t *testing.T) {
	// below threshold still true; above threshold nil probs → 0% not invent 50
	if !MoreTypesProbabilitySess(testAmbientSession, NewRngSess(testAmbientSession, 1), nil, 5) {
		t.Fatal("count<10 always true")
	}
	for seed := uint64(1); seed < 40; seed++ {
		if MoreTypesProbabilitySess(testAmbientSession, NewRngSess(testAmbientSession, seed), nil, 20) {
			t.Fatal("nil probs past threshold must not invent 50% true")
		}
	}
}
