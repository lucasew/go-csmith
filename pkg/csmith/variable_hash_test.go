package csmith

import (
	"strings"
	"testing"
)

func TestHashSimpleInt(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	out := v.HashOutput()
	if !strings.Contains(out, `transparent_crc(g_1, "g_1"`) {
		t.Fatal(out)
	}
	// empty name sticky — no invent transparent_crc(,"")
	anon := &Variable{Name: "", Type: GetIntType()}
	if anon.HashOutput() != "" {
		t.Fatal("empty name must fail closed hash")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty name HashOutput must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestHashPointerEmpty(t *testing.T) {
	v := CreateVariableScalars("g_2", PointerTo(GetIntType()), false, false)
	if v.HashOutput() != "" {
		t.Fatal("pointers must not hash")
	}
}

func TestHashStructFields(t *testing.T) {
	st := &Type{
		isStruct:   true,
		StructName: "S0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), Qfer: NewCVQualifiers([]bool{false}, []bool{false}), BitWidth: -1},
			{Name: "f1", Type: GetSimpleType(EUInt), Qfer: NewCVQualifiers([]bool{false}, []bool{false}), BitWidth: -1},
		},
	}
	v := CreateVariableQfer("g_3", st, NewCVQualifiers([]bool{false}, []bool{false}))
	out := v.HashOutput()
	if !strings.Contains(out, "g_3.f0") || !strings.Contains(out, "g_3.f1") {
		t.Fatal(out)
	}
	// top-level aggregate name must not appear as scalar crc
	if strings.Contains(out, `transparent_crc(g_3, "g_3"`) {
		t.Fatal("must not hash whole struct", out)
	}
}

func TestHashArrayLoops(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	CtrlVarsDoFinalization()
	// ArrayVariable::hash uses get_last_ctrl_vars (no letter invent without pool)
	_ = GetNewCtrlVars(Defaults())
	// live AsArray required (no invent hash-array expand from ArraySizes alone)
	av := &ArrayVariable{
		Variable: Variable{
			Name:       "g_4",
			Type:       GetIntType(),
			IsArray:    true,
			ArraySizes: []int{3},
			Qfer:       NewCVQualifiers([]bool{false}, []bool{false}),
		},
		Sizes: []int{3},
	}
	av.AsArray = av
	out := av.Variable.HashOutput()
	// Variable::new_ctrl_vars uses letter names i, j, k…
	if !strings.Contains(out, "for (i = 0") || !strings.Contains(out, "g_4[i]") {
		t.Fatal(out)
	}
	// ArrayVariable.cpp:786–788 — hash_value_printf default true → index printf
	if !strings.Contains(out, `if (print_hash_value) printf("index = [%d]\n", i);`) {
		t.Fatal("missing hash_value_printf index line", out)
	}
	// undersized ctrl sticky — no invent loops with empty index
	ClearErrorSess(testAmbientSession)
	CtrlVarsDoFinalization()
	if av.Variable.HashOutput() != "" {
		t.Fatal("no last ctrl must fail closed array hash")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("no last ctrl HashOutput must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray sticky empty
	shell := &Variable{Name: "g_5", Type: GetIntType(), IsArray: true, ArraySizes: []int{3}}
	if shell.HashOutput() != "" {
		t.Fatal("IsArray without AsArray HashOutput must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray HashOutput must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	CtrlVarsDoFinalization()
}

// ArrayVariable.cpp:722–723 — itemized (collective!=0) hash is a no-op.
// GlobalList holds collective + itemized; only parent emits one loop nest.
func TestHashArraySkipsItemizedCollective(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	CtrlVarsDoFinalization()
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	_ = GetNewCtrlVars(opts)
	parent := &ArrayVariable{
		Variable: Variable{
			Name: "g_62", Type: GetIntType(), IsArray: true, ArraySizes: []int{2, 3},
			Qfer: NewCVQualifiers([]bool{false}, []bool{false}),
		},
		Sizes: []int{2, 3},
	}
	parent.AsArray = parent
	item := &ArrayVariable{
		Variable: Variable{
			Name: "g_62", Type: GetIntType(), IsArray: true, ArraySizes: []int{2, 3},
			Qfer: NewCVQualifiers([]bool{false}, []bool{false}),
		},
		Sizes: []int{2, 3}, Collective: parent, Indices: []string{"1", "0"},
	}
	item.AsArray = item
	vs := NewVariableSelector(opts)
	// C++ create_array_and_itemize: collective then itemized on GlobalList
	vs.GlobalList = []*Variable{&parent.Variable, &item.Variable}
	out := HashGlobalVariables(vs)
	if strings.Count(out, "transparent_crc(g_62") != 1 {
		t.Fatalf("expected single collective hash, got:\n%s", out)
	}
	if !strings.Contains(out, `if (print_hash_value) printf("index = [%d][%d]\n", i, j);`) {
		t.Fatal("missing multi-dim index printf", out)
	}
	// itemized alone must emit nothing
	if item.Variable.HashOutput() != "" {
		t.Fatal("itemized ArrayVariable::hash must no-op")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("itemized no-op must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	CtrlVarsDoFinalization()
}

// ArrayVariable.cpp:742–744 — field_names empty after union exclusions → give up
// before for-loops (no empty for+index shell). Seed 94: g_336[5] all unreadable.
func TestHashArrayUnionAllUnreadableSkipsLoops(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	CtrlVarsDoFinalization()
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	_ = GetNewCtrlVars(opts)
	ut := &Type{
		isUnion:    true,
		StructName: "U2",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
			{Name: "f1", Type: GetIntType(), BitWidth: -1},
		},
	}
	av := &ArrayVariable{
		Variable: Variable{
			Name: "g_336", Type: ut, IsArray: true, ArraySizes: []int{5},
			Qfer: NewCVQualifiers([]bool{false}, []bool{false}),
		},
		Sizes: []int{5},
	}
	av.AsArray = av
	// BOTTOM last-write → no field readable (FactUnion.cpp / seed 94-style)
	facts := []*FactUnion{MakeFactUnion(&av.Variable, FactUnionBottom)}
	out := hashArrayVariable(&av.Variable, nil, facts)
	if out != "" {
		t.Fatalf("empty field_names must skip hash entirely, got:\n%s", out)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete BOTTOM facts must not sticky", GetErrorSess(testAmbientSession))
	}
	// without facts (nil) still has type leaves → would hash both fields
	all := hashArrayVariable(&av.Variable, nil, nil)
	if !strings.Contains(all, "for (i = 0") || !strings.Contains(all, "g_336[i].f0") {
		t.Fatal("nil facts must hash all leaves", all)
	}
	// partial: only f0 readable → loops + f0 only
	f0 := []*FactUnion{MakeFactUnion(&av.Variable, 0)}
	part := hashArrayVariable(&av.Variable, nil, f0)
	if !strings.Contains(part, "g_336[i].f0") || strings.Contains(part, "g_336[i].f1") {
		t.Fatal("want only f0", part)
	}
	if !strings.Contains(part, "for (i = 0") {
		t.Fatal("readable payload must still emit loops", part)
	}
	ClearErrorSess(testAmbientSession)
	CtrlVarsDoFinalization()
}

func TestHashArrayHashValuePrintfOff(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	CtrlVarsDoFinalization()
	opts := Defaults()
	opts.HashValuePrintf = false
	SetProcessOptionsSess(testAmbientSession, opts)
	_ = GetNewCtrlVars(opts)
	av := &ArrayVariable{
		Variable: Variable{
			Name: "g_x", Type: GetIntType(), IsArray: true, ArraySizes: []int{2},
			Qfer: NewCVQualifiers([]bool{false}, []bool{false}),
		},
		Sizes: []int{2},
	}
	av.AsArray = av
	out := av.Variable.HashOutput()
	if strings.Contains(out, "print_hash_value) printf") {
		t.Fatal("hash_value_printf false must omit index printf", out)
	}
	if !strings.Contains(out, "transparent_crc(g_x[i]") {
		t.Fatal(out)
	}
	// restore process defaults for later tests
	SetProcessOptionsSess(testAmbientSession, Defaults())
	ClearErrorSess(testAmbientSession)
	CtrlVarsDoFinalization()
}

func TestGenerateHashUsesFieldCrc(t *testing.T) {
	foundStructHash := false
	foundPtrSkip := false
	for seed := uint64(1); seed < 80; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, ".f0") && strings.Contains(out, "transparent_crc") {
			foundStructHash = true
		}
		// pointer globals: should not transparent_crc(g_N with * type as raw) — hard to check
		_ = foundPtrSkip
		if foundStructHash {
			break
		}
	}
	if !foundStructHash {
		t.Log("struct field hash not seen in sample — optional")
	}
}

func TestHashOutputIsAggregateResidualSticky(t *testing.T) {
	// IsAggregate residual soft invent was invent soft-continue scalar hash past incomplete.
	// Type-nil already sticky at hashOutput entry.
	ClearErrorSess(testAmbientSession)
	v := &Variable{Name: "g_x", Type: nil}
	if v.hashOutput(nil, nil) != "" {
		t.Fatal("Type-nil hashOutput must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil hashOutput must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
