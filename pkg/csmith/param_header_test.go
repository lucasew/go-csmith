package csmith

import (
	"strings"
	"testing"
)

func TestGenerateParameterVariableArgStructsOff(t *testing.T) {
	ClearError()
	opts := Defaults()
	opts.ArgStructs = false
	opts.ArgUnions = false
	vs := NewVariableSelector(opts)
	// force type env with a struct
	st := &Type{
		isStruct:   true,
		StructName: "S0",
		Fields:     []StructField{{Name: "f0", Type: GetIntType(), BitWidth: -1}},
	}
	vs.Types = &TypeEnv{
		AllTypes:    []*Type{st, GetIntType()},
		StructTypes: []*Type{st},
	}
	// many draws: never struct/union param when disabled
	for seed := uint64(1); seed < 40; seed++ {
		f := &Function{Name: "f"}
		// force non-pointer path by empty derived
		vs.Types.DerivedTypes = nil
		_ = vs.GenerateParameterVariable(f, NewRng(seed))
		if len(f.Param) == 0 {
			continue
		}
		p := f.Param[0]
		if p.Type != nil && (p.Type.IsStruct() || p.Type.IsUnion()) {
			t.Fatalf("seed %d: got aggregate param %s", seed, p.Type.CName())
		}
	}
}

func TestGenerateParameterVariablePointerChoice(t *testing.T) {
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	pt := PointerTo(GetIntType())
	vs.Types = &TypeEnv{DerivedTypes: []*Type{pt}, AllTypes: []*Type{GetIntType(), pt}}
	foundPtr := false
	for seed := uint64(1); seed < 50; seed++ {
		ClearError()
		f := &Function{Name: "f"}
		_ = vs.GenerateParameterVariable(f, NewRng(seed))
		if len(f.Param) > 0 && f.Param[0].Type != nil && f.Param[0].Type.IsPointerLike() {
			foundPtr = true
			break
		}
	}
	if !foundPtr {
		t.Fatal("expected some pointer params")
	}
}

func TestGenerateParameterVariableNoMakePointerInvent(t *testing.T) {
	// VariableSelector.cpp:968–970 — only choose_random_pointer_type; no make invent
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	// HasPointerType true via DerivedTypes but ChooseRandomPointerType needs non-empty
	// empty Derived after flip: HasPointerType false → nonvoid path
	vs.Types = &TypeEnv{AllTypes: []*Type{GetIntType()}}
	f := &Function{Name: "f"}
	v := vs.GenerateParameterVariable(f, NewRng(1))
	if v == nil || v.Type == nil {
		t.Fatal("expected nonvoid param")
	}
	if v.Type.IsPointerLike() {
		t.Fatal("no derived pointers → must not invent pointer param")
	}
	// nil Types → fail closed (no GetSimpleType invent)
	vs2 := NewVariableSelector(opts)
	vs2.Types = nil
	if vs2.GenerateParameterVariable(&Function{Name: "g"}, NewRng(2)) != nil {
		t.Fatal("nil Types must not invent simple param")
	}
}

func TestOutputHeaderForbiddenReturnStructFailClosed(t *testing.T) {
	// Function.cpp:517–518 — assert when !return_structs and eStruct return
	st := &Type{isStruct: true, StructName: "S", Fields: []StructField{{Name: "x", Type: GetIntType(), BitWidth: -1}}}
	f := &Function{Name: "f", ReturnType: st}
	opts := Defaults()
	opts.ReturnStructs = false
	if f.OutputHeaderOpts(false, opts) != "" {
		t.Fatal("struct return with ReturnStructs off must fail closed")
	}
}

func TestOutputHeaderForbiddenArgStructFailClosed(t *testing.T) {
	// Function.cpp:489–490 — assert when !arg_structs and struct param
	st := &Type{isStruct: true, StructName: "S", Fields: []StructField{{Name: "x", Type: GetIntType(), BitWidth: -1}}}
	pv := &Variable{Name: "p", Type: st}
	f := &Function{Name: "f", ReturnType: GetIntType(), Param: []*Variable{pv}}
	opts := Defaults()
	opts.ArgStructs = false
	if f.OutputHeaderOpts(false, opts) != "" {
		t.Fatal("struct arg with ArgStructs off must fail closed")
	}
}

func TestParamListNoInventEmptyNameOrType(t *testing.T) {
	// Function.cpp param Output — live type + name; no invent "int " / bare type
	f := &Function{
		Name:       "func_p",
		ReturnType: GetIntType(),
		Param: []*Variable{
			{Name: "", Type: GetIntType(), Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		},
	}
	if out := f.OutputHeader(false); out != "" {
		t.Fatal("empty param name must fail closed header", out)
	}
	f.Param[0].Name = "p_1"
	f.Param[0].Type = nil
	if out := f.OutputHeader(false); out != "" {
		t.Fatal("nil param type must fail closed header", out)
	}
}

func TestOutputHeaderInlineStatic(t *testing.T) {
	f := &Function{
		Name:       "func_2",
		ReturnType: GetIntType(),
		IsInlined:  true,
		Param: []*Variable{
			CreateVariableScalars("p_1", GetIntType(), false, false),
		},
	}
	h := f.OutputHeader(true)
	if !strings.Contains(h, "inline ") || !strings.Contains(h, "static ") {
		t.Fatal(h)
	}
	if !strings.Contains(h, "func_2(p_1") && !strings.Contains(h, "func_2(") {
		t.Fatal(h)
	}
	// empty params → void
	f2 := &Function{Name: "f", ReturnType: GetIntType()}
	if !strings.Contains(f2.OutputHeader(false), "(void)") {
		t.Fatal(f2.OutputHeader(false))
	}
}

func TestOutputForwardDeclUsesHeader(t *testing.T) {
	f := &Function{Name: "g_1", ReturnType: GetIntType(), IsInlined: true}
	d := f.OutputForwardDeclOpts(true, nil, false)
	if !strings.HasSuffix(d, ";") {
		t.Fatal(d)
	}
	if !strings.Contains(d, "inline ") || !strings.Contains(d, "static ") {
		t.Fatal(d)
	}
}

func TestOutputHeaderAlias(t *testing.T) {
	f := &Function{Name: "func_3", AliasName: "func_3_alias", ReturnType: GetIntType()}
	a := f.OutputHeaderAlias(true)
	if !strings.Contains(a, `alias("func_3")`) || !strings.Contains(a, "static ") {
		t.Fatal(a)
	}
	// forward decl ends with ;
	if !strings.HasSuffix(f.OutputForwardDeclAlias(false), ";") {
		t.Fatal(f.OutputForwardDeclAlias(false))
	}
}
