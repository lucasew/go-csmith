package csmith

import (
	"strings"
	"testing"
)

func TestGenerateParameterVariableArgStructsOff(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.ArgStructs = false
	opts.ArgUnions = false
	vs := NewVariableSelector(testAmbientSession, opts)
	// force type env with a struct
	st := &Type{
		isStruct:   true,
		StructName: "S0",
		Fields:     []StructField{{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1}},
	}
	vs.Types = &TypeEnv{Sess: testAmbientSession,
		AllTypes:    []*Type{st, GetIntTypeSess(testAmbientSession)},
		StructTypes: []*Type{st},
	}
	// many draws: never struct/union param when disabled
	for seed := uint64(1); seed < 40; seed++ {
		f := &Function{Name: "f"}
		// force non-pointer path by empty derived
		vs.Types.DerivedTypes = nil
		_ = vs.GenerateParameterVariable(f, NewRngSess(testAmbientSession, seed))
		if len(f.Param) == 0 {
			continue
		}
		p := f.Param[0]
		if p.Type != nil && (p.Type.IsStructSess(testAmbientSession) || p.Type.IsUnionSess(testAmbientSession)) {
			t.Fatalf("seed %d: got aggregate param %s", seed, p.Type.CNameSess(testAmbientSession))
		}
	}
}

func TestGenerateParameterVariablePointerChoice(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	pt := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	vs.Types = &TypeEnv{Sess: testAmbientSession, DerivedTypes: []*Type{pt}, AllTypes: []*Type{GetIntTypeSess(testAmbientSession), pt}}
	foundPtr := false
	for seed := uint64(1); seed < 50; seed++ {
		ClearErrorSess(testAmbientSession)
		f := &Function{Name: "f"}
		_ = vs.GenerateParameterVariable(f, NewRngSess(testAmbientSession, seed))
		if len(f.Param) > 0 && f.Param[0].Type != nil && f.Param[0].Type.IsPointerLikeSess(testAmbientSession) {
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
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	// HasPointerType true via DerivedTypes but ChooseRandomPointerType needs non-empty
	// empty Derived after flip: HasPointerType false → nonvoid path
	vs.Types = &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetIntTypeSess(testAmbientSession)}}
	f := &Function{Name: "f"}
	v := vs.GenerateParameterVariable(f, NewRngSess(testAmbientSession, 1))
	if v == nil || v.Type == nil {
		t.Fatal("expected nonvoid param")
	}
	if v.Type.IsPointerLikeSess(testAmbientSession) {
		t.Fatal("no derived pointers → must not invent pointer param")
	}
	// nil Types → fail closed (no GetSimpleType invent)
	vs2 := NewVariableSelector(testAmbientSession, opts)
	vs2.Types = nil
	if vs2.GenerateParameterVariable(&Function{Name: "g"}, NewRngSess(testAmbientSession, 2)) != nil {
		t.Fatal("nil Types must not invent simple param")
	}
}

func TestOutputHeaderForbiddenReturnStructFailClosed(t *testing.T) {
	// Function.cpp:517–518 — assert when !return_structs and eStruct return sticky
	ClearErrorSess(testAmbientSession)
	st := &Type{isStruct: true, StructName: "S", Fields: []StructField{{Name: "x", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1}}}
	f := &Function{Name: "f", ReturnType: st}
	opts := Defaults()
	opts.ReturnStructs = false
	if f.OutputHeaderOptsSess(testAmbientSession, false, opts) != "" {
		t.Fatal("struct return with ReturnStructs off must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("forbidden return struct must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputHeaderForbiddenArgStructFailClosed(t *testing.T) {
	// Function.cpp:489–490 — assert when !arg_structs and struct param sticky
	ClearErrorSess(testAmbientSession)
	st := &Type{isStruct: true, StructName: "S", Fields: []StructField{{Name: "x", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1}}}
	pv := &Variable{Name: "p", Type: st}
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession), Param: []*Variable{pv}}
	opts := Defaults()
	opts.ArgStructs = false
	if f.OutputHeaderOptsSess(testAmbientSession, false, opts) != "" {
		t.Fatal("struct arg with ArgStructs off must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("forbidden arg struct must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestParamListNoInventEmptyNameOrType(t *testing.T) {
	// Function.cpp param Output — live type + name; sticky no invent "int " / bare type
	ClearErrorSess(testAmbientSession)
	f := &Function{
		Name:       "func_p",
		ReturnType: GetIntTypeSess(testAmbientSession),
		Param: []*Variable{
			{Name: "", Type: GetIntTypeSess(testAmbientSession), Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		},
	}
	if out := f.OutputHeaderSess(testAmbientSession, false); out != "" {
		t.Fatal("empty param name must fail closed header", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty param name must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	f.Param[0].Name = "p_1"
	f.Param[0].Type = nil
	if out := f.OutputHeaderSess(testAmbientSession, false); out != "" {
		t.Fatal("nil param type must fail closed header", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil param type must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputHeaderInlineStatic(t *testing.T) {
	f := &Function{
		Name:       "func_2",
		ReturnType: GetIntTypeSess(testAmbientSession),
		IsInlined:  true,
		Param: []*Variable{
			CreateVariableScalarsSess(testAmbientSession, "p_1", GetIntTypeSess(testAmbientSession), false, false),
		},
	}
	h := f.OutputHeaderSess(testAmbientSession, true)
	if !strings.Contains(h, "inline ") || !strings.Contains(h, "static ") {
		t.Fatal(h)
	}
	if !strings.Contains(h, "func_2(p_1") && !strings.Contains(h, "func_2(") {
		t.Fatal(h)
	}
	// empty params → void
	f2 := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	if !strings.Contains(f2.OutputHeaderSess(testAmbientSession, false), "(void)") {
		t.Fatal(f2.OutputHeaderSess(testAmbientSession, false))
	}
}

func TestOutputForwardDeclUsesHeader(t *testing.T) {
	f := &Function{Name: "g_1", ReturnType: GetIntTypeSess(testAmbientSession), IsInlined: true}
	d := f.OutputForwardDeclSess(testAmbientSession, true, nil, false)
	if !strings.HasSuffix(d, ";") {
		t.Fatal(d)
	}
	if !strings.Contains(d, "inline ") || !strings.Contains(d, "static ") {
		t.Fatal(d)
	}
}

func TestOutputHeaderAlias(t *testing.T) {
	f := &Function{Name: "func_3", AliasName: "func_3_alias", ReturnType: GetIntTypeSess(testAmbientSession)}
	a := f.OutputHeaderAliasSess(testAmbientSession, true)
	if !strings.Contains(a, `alias("func_3")`) || !strings.Contains(a, "static ") {
		t.Fatal(a)
	}
	// forward decl ends with ;
	if !strings.HasSuffix(f.OutputForwardDeclAliasSess(testAmbientSession, false), ";") {
		t.Fatal(f.OutputForwardDeclAliasSess(testAmbientSession, false))
	}
}

func TestOutputHeaderReturnStructOptionResidualSticky(t *testing.T) {
	// IsStruct residual soft invent was invent scalar header past ReturnStructs off.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.ReturnStructs = false
	st := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	f := &Function{Name: "func_1", ReturnType: st}
	if s := f.OutputHeaderOptsSess(testAmbientSession, false, opts); s != "" {
		t.Fatal("ReturnStructs off must fail closed OutputHeader", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("ReturnStructs off OutputHeader must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestParamListArgStructOptionResidualSticky(t *testing.T) {
	// IsStruct residual soft invent was invent param list past ArgStructs off.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.ArgStructs = false
	st := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	p := &Variable{Name: "p", Type: st, Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession), Param: []*Variable{p}}
	if s := f.paramListCOptsSess(testAmbientSession, opts); s != "" {
		t.Fatal("ArgStructs off must fail closed paramListC", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("ArgStructs off paramListC must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
