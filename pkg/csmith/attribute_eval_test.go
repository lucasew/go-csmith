package csmith

import (
	"strconv"
	"strings"
	"testing"
)

func TestBooleanAttribute(t *testing.T) {
	a := &BooleanAttribute{Name: "unused", Prob: 100}
	if a.MakeRandom(NewRng(1)) != "unused" {
		t.Fatal("always")
	}
	a.Prob = 0
	if a.MakeRandom(NewRng(1)) != "" {
		t.Fatal("never")
	}
}

func TestAttributeGeneratorOutput(t *testing.T) {
	g := &AttributeGenerator{Attributes: []Attribute{
		&BooleanAttribute{Name: "unused", Prob: 100},
		&BooleanAttribute{Name: "used", Prob: 100},
	}}
	out := g.Output(NewRng(1))
	if !strings.Contains(out, "__attribute__((") || !strings.Contains(out, "unused") {
		t.Fatal(out)
	}
	if !strings.HasSuffix(out, "))") {
		t.Fatal(out)
	}
}

func TestMultiChoiceAttribute(t *testing.T) {
	// Attribute.cpp:66 — name("choice") with quotes
	a := &MultiChoiceAttribute{Name: "visibility", Prob: 100, Choices: []string{"default", "hidden"}}
	s := a.MakeRandom(NewRng(2))
	if !strings.HasPrefix(s, "visibility(\"") || !strings.HasSuffix(s, "\")") {
		t.Fatal(s)
	}
}

func TestAlignedAttribute(t *testing.T) {
	// Attribute.cpp:82–84 — aligned(1<<k)
	a := &AlignedAttribute{Name: "aligned", Prob: 100, Alignment: 4}
	s := a.MakeRandom(NewRng(2))
	if !strings.HasPrefix(s, "aligned(") || !strings.HasSuffix(s, ")") {
		t.Fatal(s)
	}
	// extract number
	inner := strings.TrimSuffix(strings.TrimPrefix(s, "aligned("), ")")
	n, err := strconv.Atoi(inner)
	if err != nil || n < 1 || (n&(n-1)) != 0 {
		t.Fatal("want power of 2", s)
	}
	// no soft invent alignment=1 when ctor left Alignment 0
	a0 := &AlignedAttribute{Name: "aligned", Prob: 100, Alignment: 0}
	if a0.MakeRandom(NewRng(1)) != "" {
		t.Fatal("Alignment 0 must not invent aligned(1)")
	}
}

func TestSectionAttributeNoInventName(t *testing.T) {
	// Attribute name from ctor; no invent "section" when empty
	a := &SectionAttribute{Name: "", Prob: 100}
	if s := a.MakeRandom(NewRng(1)); s != "" {
		t.Fatal("empty name must fail closed", s)
	}
}

func TestAttributeNoInventEmptyName(t *testing.T) {
	// Boolean / MultiChoice / Aligned require live name from ctor
	if s := (&BooleanAttribute{Name: "", Prob: 100}).MakeRandom(NewRng(1)); s != "" {
		t.Fatal("boolean empty name", s)
	}
	if s := (&MultiChoiceAttribute{Name: "", Prob: 100, Choices: []string{"a"}}).MakeRandom(NewRng(1)); s != "" {
		t.Fatal("multichoice empty name", s)
	}
	if s := (&AlignedAttribute{Name: "", Prob: 100, Alignment: 4}).MakeRandom(NewRng(1)); s != "" {
		t.Fatal("aligned empty name", s)
	}
	// empty choice slot — no invent visibility("")
	if s := (&MultiChoiceAttribute{Name: "visibility", Prob: 100, Choices: []string{""}}).MakeRandom(NewRng(1)); s != "" {
		t.Fatal("empty choice must fail closed", s)
	}
}

func TestSectionAttribute(t *testing.T) {
	a := &SectionAttribute{Name: "section", Prob: 100}
	s := a.MakeRandom(NewRng(3))
	if !strings.HasPrefix(s, "section(\"usersection") || !strings.HasSuffix(s, "\")") {
		t.Fatal(s)
	}
}

func TestNewVarAttrGeneratorGated(t *testing.T) {
	opts := Defaults()
	opts.VariableAttributes = false
	g := NewVarAttrGenerator(opts, NewProbabilities(opts))
	if len(g.Attributes) != 0 {
		t.Fatal("off")
	}
	opts.VariableAttributes = true
	g = NewVarAttrGenerator(opts, NewProbabilities(opts))
	if len(g.Attributes) < 6 {
		t.Fatal(len(g.Attributes))
	}
	// first is visibility multi
	if _, ok := g.Attributes[0].(*MultiChoiceAttribute); !ok {
		t.Fatalf("%T", g.Attributes[0])
	}
	if _, ok := g.Attributes[1].(*AlignedAttribute); !ok {
		t.Fatalf("%T", g.Attributes[1])
	}
	// nil probs → 0% (no invent default 30)
	g0 := NewVarAttrGenerator(opts, nil)
	if len(g0.Attributes) == 0 {
		t.Fatal("attrs present at 0%")
	}
	if ba, ok := g0.Attributes[2].(*BooleanAttribute); !ok || ba.Prob != 0 {
		t.Fatalf("nil probs must not invent Prob=30, got %#v", g0.Attributes[2])
	}
}

func TestNewFuncAttrGeneratorHasSection(t *testing.T) {
	opts := Defaults()
	opts.FunctionAttributes = true
	g := NewFuncAttrGenerator(opts, NewProbabilities(opts))
	foundSec, foundAlign := false, false
	for _, a := range g.Attributes {
		if _, ok := a.(*SectionAttribute); ok {
			foundSec = true
		}
		if al, ok := a.(*AlignedAttribute); ok && al.Name == "aligned" {
			foundAlign = true
		}
	}
	if !foundSec || !foundAlign {
		t.Fatal("section/aligned", foundSec, foundAlign)
	}
	// nil probs → 0% (no invent default 30)
	g0 := NewFuncAttrGenerator(opts, nil)
	for _, a := range g0.Attributes {
		if ba, ok := a.(*BooleanAttribute); ok && ba.Prob != 0 {
			t.Fatalf("nil probs must not invent Prob=30, got %s=%d", ba.Name, ba.Prob)
		}
	}
}


func TestTypeAttrOnStructDecl(t *testing.T) {
	st := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	attrs := &AttributeGenerator{Attributes: []Attribute{
		&BooleanAttribute{Name: "unused", Prob: 100},
	}}
	out := st.OutputStructDeclOpts(NewRng(1), attrs)
	if !strings.Contains(out, "struct S0") || !strings.Contains(out, "unused") {
		t.Fatal(out)
	}
}

func TestGetEvalToSubexpsComma(t *testing.T) {
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	e := &Expression{
		Term:     TermCommaExpr,
		CommaLHS: &Expression{Term: TermVariable, Var: a, ExprType: GetIntType()},
		CommaRHS: &Expression{Term: TermVariable, Var: b, ExprType: GetIntType()},
	}
	subs := GetEvalToSubexps(e)
	if len(subs) != 1 || subs[0].Var != b {
		t.Fatal(subs)
	}
}

func TestGetEvalToSubexpsIncompleteFailClosed(t *testing.T) {
	// incomplete IR must not invent empty eval list as complete (overlap skip)
	if GetEvalToSubexps(&Expression{Term: TermCommaExpr}) != nil {
		t.Fatal("nil CommaRHS must fail closed")
	}
	if GetEvalToSubexps(&Expression{Term: TermAssignment}) != nil {
		t.Fatal("nil Assign must fail closed")
	}
	if GetEvalToSubexps(&Expression{Term: TermVariable}) != nil {
		t.Fatal("nil Var must fail closed")
	}
	if GetEvalToSubexps(&Expression{Term: TermFunction}) != nil {
		t.Fatal("nil Invoke must fail closed")
	}
	if GetEvalToSubexps(&Expression{Term: TermConstant}) != nil {
		t.Fatal("nil Con must fail closed")
	}
}

func TestHaveOverlappingFieldsUnion(t *testing.T) {
	ut := &Type{isUnion: true, StructName: "U0", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	f0 := &Variable{Name: "g_u.f0", Type: GetIntType(), FieldVarOf: uv}
	f1 := &Variable{Name: "g_u.f1", Type: GetIntType(), FieldVarOf: uv}
	uv.FieldVars = []*Variable{f0, f1}
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	facts := []*FactPointTo{MakeFactPointToSet(p, []*Variable{f0, f1})}
	// indirection: Var type *int, ExprType int → level 1
	e1 := &Expression{Term: TermVariable, Var: p, ExprType: GetIntType()}
	e2 := &Expression{Term: TermVariable, Var: p, ExprType: GetIntType()}
	u1 := FindUnionPointees(facts, e1)
	if len(u1) == 0 {
		t.Fatalf("expected union pointees, ind=%d", e1.IndirectLevel())
	}
	if !HaveOverlappingFields(e1, e2, facts) {
		t.Fatal("overlap", u1)
	}
}

func TestHaveOverlappingFieldsIncompleteFailClosed(t *testing.T) {
	// soft invent: FindUnionPointees nil → len==0 → no overlap success
	// fair: incomplete facts/pointees fail closed as overlap
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	e1 := &Expression{Term: TermVariable, Var: p, ExprType: GetIntType()}
	e2 := &Expression{Term: TermVariable, Var: p, ExprType: GetIntType()}
	holeFacts := []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	if !HaveOverlappingFields(e1, e2, holeFacts) {
		t.Fatal("incomplete facts must fail closed as overlap")
	}
	if FindUnionPointees(holeFacts, e1) != nil {
		t.Fatal("FindUnionPointees incomplete must be nil not empty")
	}
	// complete empty: non-pointer term → empty unions, no overlap
	c := &Expression{Term: TermConstant, Con: MakeInt(1)}
	empty := FindUnionPointees(nil, c)
	if empty == nil || len(empty) != 0 {
		t.Fatal("complete non-ptr must be non-nil empty", empty)
	}
	if HaveOverlappingFields(c, c, nil) {
		t.Fatal("complete constants must not invent overlap")
	}
}

func TestBuiltinOutputSkipped(t *testing.T) {
	f := &Function{Name: "__builtin_clz", ReturnType: GetIntType(), IsBuiltin: true}
	if f.Output() != "" || f.OutputForwardDecl() != "" {
		t.Fatal("builtin emit")
	}
}

func TestVisitFactsReturnDeadPtr(t *testing.T) {
	opts := Defaults()
	opts.NoReturnDeadPointer = true
	f := &Function{Name: "f", ReturnType: PointerTo(GetIntType())}
	f.RV = CreateVariableScalars("f_rv", PointerTo(GetIntType()), false, false)
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	blk := &Block{Func: f, LocalVars: []*Variable{loc}}
	f.Stack = []*Block{blk}
	lp := CreateVariableScalars("l_p", PointerTo(GetIntType()), false, false)
	fm := NewFactMgr(f)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(lp, loc)}
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	st := Stmt{
		Kind: StmtReturn,
		Expr: &Expression{Term: TermVariable, Var: lp, ExprType: PointerTo(GetIntType())},
	}
	if VisitFactsStatementReturn(&st, &cg, opts) {
		t.Fatal("should reject local-pointing return")
	}
}

func TestVisitFactsLhsCompoundRead(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	lhs := &Lhs{Var: v, Type: GetIntType(), CompoundAssign: true}
	cg := EmptyCGContext()
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if !cg.VisitFactsLhs(lhs, Defaults()) {
		t.Fatal("compound")
	}
	// compound should have read then write
	if !eff.IsRead(v) || !eff.IsWritten(v) {
		t.Fatal("rw", eff.IsRead(v), eff.IsWritten(v))
	}
}
