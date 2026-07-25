package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomStructType(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
	// Type.cpp GenerateSimpleTypes before make_random_struct_type
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	r := NewRng(2)
	st := MakeRandomStructType(r, opts, probs, &env, "S0")
	if st == nil || !st.IsStruct() || len(st.Fields) < 1 {
		t.Fatal(st)
	}
	// Type.cpp:1088–1091 — make_random_struct_type leaves used=false until choose_random*
	if st.Used {
		t.Fatal("make_random_struct_type must not invent used=true at create")
	}
	// Type.cpp:298–302 — shared sid sequence; first aggregate → S0
	if st.StructName != "S0" || st.SID != 0 {
		t.Fatalf("shared sid name want S0/0 got %q sid=%d", st.StructName, st.SID)
	}
	if len(env.StructTypes) != 1 {
		t.Fatal(env.StructTypes)
	}
	for _, f := range st.Fields {
		if f.Type == nil {
			t.Fatal("no nil-type field invent")
		}
	}
	// mark used for emit contract tests
	st.Used = true
	decl := st.OutputStructDecl()
	if !strings.Contains(decl, "struct S0") || !strings.Contains(decl, "f0") {
		t.Fatal(decl)
	}
}

// Type.cpp:658–666 make_one_struct_field must not mark nested field types used
// (only Type::choose_random does). Regression: seed 56 emitted unused S1.
func TestMakeOneStructFieldDoesNotMarkUsed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := &TypeEnv{Sess: testAmbientSession}
	// Seed AllTypes with simples + one unused prior struct.
	s0 := &Type{
		isStruct: true, StructName: "S0", SID: 0, Used: false,
		Fields: []StructField{{Name: "f0", Type: GetIntType(), BitWidth: -1}},
	}
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EUInt), s0}
	env.StructTypes = []*Type{s0}
	// Many field picks; S0 may be chosen as nested type — must stay unused.
	for seed := uint64(1); seed < 80; seed++ {
		ClearErrorSess(testAmbientSession)
		s0.Used = false
		f := MakeOneStructField(NewRng(seed), opts, probs, env, 0)
		if f.Type == nil {
			continue
		}
		if s0.Used {
			t.Fatalf("seed %d: make_one_struct_field must not mark nested S0 used", seed)
		}
	}
	// choose_random still marks used when it picks the type
	ClearErrorSess(testAmbientSession)
	s0.Used = false
	picked := false
	for seed := uint64(1); seed < 200 && !picked; seed++ {
		s0.Used = false
		ty := env.ChooseRandom(NewRng(seed), opts, probs, false)
		if ty == s0 {
			if !s0.Used {
				t.Fatal("ChooseRandom must mark picked struct used")
			}
			picked = true
		}
	}
}

func TestAggregateSharedSIDSequence(t *testing.T) {
	// Type.cpp:298–302 + 1675–1678 — one static sequence for struct and union tags.
	// After S0, first union is U1 (not U0).
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
	env.AllTypes = []*Type{GetIntType()}
	st := MakeRandomStructType(NewRng(2), opts, probs, &env, "")
	if st == nil || st.StructName != "S0" {
		t.Fatalf("struct: %v name=%q", st, func() string {
			if st == nil {
				return ""
			}
			return st.StructName
		}())
	}
	ut := MakeRandomUnionType(NewRng(3), opts, probs, &env, "")
	if ut == nil || ut.StructName != "U1" || ut.SID != 1 {
		t.Fatalf("union after S0 must be U1 sid=1, got %v name=%q sid=%d", ut,
			func() string {
				if ut == nil {
					return ""
				}
				return ut.StructName
			}(), func() int {
				if ut == nil {
					return -1
				}
				return ut.SID
			}())
	}
	if env.AggregateSeq != 2 {
		t.Fatalf("AggregateSeq want 2 got %d", env.AggregateSeq)
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputStructDeclPackPragmaNonCComp(t *testing.T) {
	// Type.cpp:1823–1829 / 1879–1883 — non-ccomp pack(push) then pack(1); pack(pop)
	ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults()) // CComp=false
	st := &Type{
		isStruct: true, StructName: "S0", Packed: true, Used: true,
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		},
	}
	decl := st.OutputStructDecl()
	if !strings.Contains(decl, "#pragma pack(push)\n#pragma pack(1)\n") {
		t.Fatalf("expected split pack(push)/pack(1), got %q", decl)
	}
	if strings.Contains(decl, "#pragma pack(push, 1)") {
		t.Fatalf("must not invent combined pack(push, 1): %q", decl)
	}
	if !strings.Contains(decl, "#pragma pack(pop)") {
		t.Fatalf("expected pack(pop): %q", decl)
	}
	// ccomp path: only pack(1) / pack()
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.CComp = true
	SetProcessOptionsSess(testAmbientSession, opts)
	decl2 := st.OutputStructDecl()
	if strings.Contains(decl2, "#pragma pack(push)") {
		t.Fatalf("ccomp must not emit pack(push): %q", decl2)
	}
	if !strings.Contains(decl2, "#pragma pack(1)\n") || !strings.Contains(decl2, "#pragma pack()\n") {
		t.Fatalf("ccomp pack(1)/pack(): %q", decl2)
	}
	SetProcessOptionsSess(testAmbientSession, Defaults())
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomStructUnionTypeNilRNGSticky(t *testing.T) {
	// Type.cpp always has process RNG; sticky no invent aggregate type shells
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
	env.AllTypes = []*Type{GetIntType()}
	if MakeRandomStructType(nil, opts, probs, &env, "S0") != nil {
		t.Fatal("nil RNG struct must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeRandomStructType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if MakeRandomUnionType(nil, opts, probs, &env, "U0") != nil {
		t.Fatal("nil RNG union must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeRandomUnionType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	opts.FixedStructFields = true
	opts.MaxStructFields = 0
	if MakeRandomStructType(NewRng(1), opts, probs, &env, "Sempty") != nil {
		t.Fatal("zero fields must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("zero-field MakeRandomStructType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGenerateAllTypesEnvCreatesStructs(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
	GenerateAllTypesEnv(NewRng(2), opts, probs, &env)
	if len(env.StructTypes) < 1 {
		t.Fatal("expected structs under MoreTypesProbability")
	}
}

func TestGenerateSimpleTypesSeedsAllNonVoid(t *testing.T) {
	// Type.cpp:1170–1176 — eChar..eUInt128 always on AllTypes (float off still listed).
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.EnableFloat = false
	opts.LongLong = false // AllowInt64 may still depend on math64
	env := TypeEnv{Sess: testAmbientSession}
	GenerateAllTypesEnv(NewRng(1), opts, NewProbabilities(opts), &env)
	// At least the 13 non-void simple slots before any struct/union.
	// (structs may append further under MoreTypesProbability.)
	found := map[ESimpleType]bool{}
	for _, ty := range env.AllTypes {
		if ty != nil && ty.IsSimple() {
			found[ty.Simple()] = true
		}
	}
	for st := EChar; int(st) < MaxSimpleTypes; st++ {
		if !found[st] {
			t.Fatalf("AllTypes missing simple %v (GenerateSimpleTypes is unfiltered)", st)
		}
	}
	if found[EVoid] {
		t.Fatal("AllTypes must not include void")
	}
}

func TestTypeGenNoInventNilRngOrProbs(t *testing.T) {
	// C++ always has RNG + Probabilities sticky; no invent fields/aggregates without them
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	if MoreTypesProbability(nil, NewProbabilities(opts), 20) {
		t.Fatal("nil RNG past threshold must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MoreTypesProbability must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if f := MakeOneStructField(nil, opts, NewProbabilities(opts), &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetIntType()}}, 0); f.Type != nil {
		t.Fatal("nil RNG MakeOneStructField must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeOneStructField must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if f := MakeOneStructField(NewRng(1), opts, nil, &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetIntType()}}, 0); f.Type != nil {
		t.Fatal("nil probs MakeOneStructField must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil probs MakeOneStructField must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if f := MakeOneUnionField(nil, opts, NewProbabilities(opts), &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetIntType()}}, 0, true); f.Type != nil {
		t.Fatal("nil RNG MakeOneUnionField must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeOneUnionField must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	env := TypeEnv{Sess: testAmbientSession}
	opts.Structs = true
	opts.Unions = true
	GenerateAllTypesEnv(nil, opts, NewProbabilities(opts), &env)
	if len(env.StructTypes) != 0 || len(env.UnionTypes) != 0 {
		t.Fatal("nil RNG must not invent aggregate types", len(env.StructTypes), len(env.UnionTypes))
	}
	if len(env.AllTypes) == 0 {
		t.Fatal("simples still seeded")
	}
	ClearErrorSess(testAmbientSession)
	// TypeEnv always live; sticky (no invent soft-skip type gen past hole)
	GenerateAllTypesEnv(NewRng(1), opts, NewProbabilities(opts), nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil env GenerateAllTypesEnv must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGenerateEmitsStructDecl(t *testing.T) {
	opts := Defaults()
	opts.Seed = 2
	out, err := Generate(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "STRUCT TYPES") && !strings.Contains(out, "struct S") {
		// may still generate
		t.Log(out[:min(500, len(out))])
	}
	if !strings.Contains(out, "struct S") {
		t.Fatal("expected struct S* in output")
	}
}

func TestMakeStructConstant(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EUInt)}
	st := MakeRandomStructType(NewRng(3), opts, probs, &env, "S0")
	if st == nil {
		t.Fatal("struct")
	}
	c := MakeStructConstant(NewRng(4), opts, probs, st)
	if c == nil || !strings.HasPrefix(c.Value, "{") {
		t.Fatal(c)
	}
	// Constant.cpp always has RNG sticky; no invent "{}" aggregate shell
	ClearErrorSess(testAmbientSession)
	if MakeStructConstant(nil, opts, probs, st) != nil {
		t.Fatal("nil RNG struct constant")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeStructConstant must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	ut := &Type{isUnion: true, StructName: "U0", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	if MakeUnionConstant(nil, opts, probs, ut) != nil {
		t.Fatal("nil RNG union constant")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeUnionConstant must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil field sticky (no invent soft-empty then ERROR_GUARD as complete miss)
	stHole := &Type{isStruct: true, StructName: "Shole", Fields: []StructField{
		{Name: "f0", Type: nil, BitWidth: -1},
	}}
	if MakeStructConstant(NewRng(5), opts, probs, stHole) != nil {
		t.Fatal("Type-nil field MakeStructConstant must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil field MakeStructConstant must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	utHole := &Type{isUnion: true, StructName: "Uhole", Fields: []StructField{
		{Name: "f0", Type: nil, BitWidth: -1},
	}}
	if MakeUnionConstant(NewRng(5), opts, probs, utHole) != nil {
		t.Fatal("Type-nil field MakeUnionConstant must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil field MakeUnionConstant must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCheckImplicitNontrivialAssignOps(t *testing.T) {
	// non-C++ complete false
	opts := Defaults()
	opts.LangCPP = false
	fields := []StructField{{Name: "f0", Type: GetIntType()}}
	if CheckImplicitNontrivialAssignOps(opts, fields) {
		t.Fatal("C mode")
	}
	// C++ with simple field → no
	opts.LangCPP = true
	if CheckImplicitNontrivialAssignOps(opts, fields) {
		t.Fatal("simple field")
	}
	// field with flag → yes
	fields = []StructField{{
		Name: "f0",
		Type: &Type{isStruct: true, StructName: "S", HasImplicitNontrivialAssignOps: true},
	}}
	if !CheckImplicitNontrivialAssignOps(opts, fields) {
		t.Fatal("flagged field")
	}
	// nil field Type sticky true (no invent no-nontrivial soft-skip past hole)
	ClearErrorSess(testAmbientSession)
	if !CheckImplicitNontrivialAssignOps(opts, []StructField{{Name: "f0", Type: nil}}) {
		t.Fatal("nil field Type must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil field Type CheckImplicitNontrivialAssignOps must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
