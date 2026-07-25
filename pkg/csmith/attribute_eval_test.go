package csmith

import (
	"strconv"
	"strings"
	"testing"
)

func TestBooleanAttribute(t *testing.T) {
	a := &BooleanAttribute{Name: "unused", Prob: 100}
	if a.MakeRandom(NewRngSess(testAmbientSession, 1)) != "unused" {
		t.Fatal("always")
	}
	a.Prob = 0
	if a.MakeRandom(NewRngSess(testAmbientSession, 1)) != "" {
		t.Fatal("never")
	}
}

func TestAttributeGeneratorOutput(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	g := &AttributeGenerator{Attributes: []Attribute{
		&BooleanAttribute{Name: "unused", Prob: 100},
		&BooleanAttribute{Name: "used", Prob: 100},
	}}
	out := g.Output(NewRngSess(testAmbientSession, 1))
	if !strings.Contains(out, "__attribute__((") || !strings.Contains(out, "unused") {
		t.Fatal(out)
	}
	if !strings.HasSuffix(out, "))") {
		t.Fatal(out)
	}
	// nil Attribute* hole sticky
	ClearErrorSess(testAmbientSession)
	gHole := &AttributeGenerator{Attributes: []Attribute{&BooleanAttribute{Name: "unused", Prob: 100}, nil}}
	if gHole.Output(NewRngSess(testAmbientSession, 1)) != "" {
		t.Fatal("nil Attribute hole must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Attribute hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// AttributeGenerator always live; sticky empty (no invent soft-skip past hole)
	if (*AttributeGenerator)(nil).Output(NewRngSess(testAmbientSession, 1)) != "" {
		t.Fatal("nil AttributeGenerator Output must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil AttributeGenerator Output must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// empty Attributes complete empty
	if (&AttributeGenerator{}).Output(NewRngSess(testAmbientSession, 1)) != "" {
		t.Fatal("empty Attributes must complete empty")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("empty Attributes must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMultiChoiceAttribute(t *testing.T) {
	// Attribute.cpp:66 — name("choice") with quotes
	a := &MultiChoiceAttribute{Name: "visibility", Prob: 100, Choices: []string{"default", "hidden"}}
	s := a.MakeRandom(NewRngSess(testAmbientSession, 2))
	if !strings.HasPrefix(s, "visibility(\"") || !strings.HasSuffix(s, "\")") {
		t.Fatal(s)
	}
}

func TestAlignedAttribute(t *testing.T) {
	// Attribute.cpp:82–84 — aligned(1<<k)
	a := &AlignedAttribute{Name: "aligned", Prob: 100, Alignment: 4}
	s := a.MakeRandom(NewRngSess(testAmbientSession, 2))
	if !strings.HasPrefix(s, "aligned(") || !strings.HasSuffix(s, ")") {
		t.Fatal(s)
	}
	// extract number
	inner := strings.TrimSuffix(strings.TrimPrefix(s, "aligned("), ")")
	n, err := strconv.Atoi(inner)
	if err != nil || n < 1 || (n&(n-1)) != 0 {
		t.Fatal("want power of 2", s)
	}
	// sticky no soft invent alignment=1 when ctor left Alignment 0
	ClearErrorSess(testAmbientSession)
	a0 := &AlignedAttribute{Name: "aligned", Prob: 100, Alignment: 0}
	if a0.MakeRandom(NewRngSess(testAmbientSession, 1)) != "" {
		t.Fatal("Alignment 0 must not invent aligned(1)")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Alignment 0 must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSectionAttributeNoInventName(t *testing.T) {
	// Attribute name from ctor; sticky no invent "section" when empty
	ClearErrorSess(testAmbientSession)
	a := &SectionAttribute{Name: "", Prob: 100}
	if s := a.MakeRandom(NewRngSess(testAmbientSession, 1)); s != "" {
		t.Fatal("empty name must fail closed", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty section name must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestAttributeNilReceiverMakeRandomSticky(t *testing.T) {
	// Attribute* always live at MakeRandom; sticky no invent "" (not-selected) past hole
	r := NewRngSess(testAmbientSession, 1)
	ClearErrorSess(testAmbientSession)
	if (*BooleanAttribute)(nil).MakeRandom(r) != "" {
		t.Fatal("nil BooleanAttribute must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil BooleanAttribute must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*MultiChoiceAttribute)(nil).MakeRandom(r) != "" {
		t.Fatal("nil MultiChoiceAttribute must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil MultiChoiceAttribute must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*AlignedAttribute)(nil).MakeRandom(r) != "" {
		t.Fatal("nil AlignedAttribute must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil AlignedAttribute must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*SectionAttribute)(nil).MakeRandom(r) != "" {
		t.Fatal("nil SectionAttribute must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil SectionAttribute must SetError sticky")
	}
	// typed-nil interface slot still hits MakeRandom sticky (not soft not-selected)
	ClearErrorSess(testAmbientSession)
	var typedNil Attribute = (*BooleanAttribute)(nil)
	if typedNil.MakeRandom(r) != "" {
		t.Fatal("typed-nil Attribute interface must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("typed-nil Attribute interface must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestAttributeNoInventEmptyName(t *testing.T) {
	// Boolean / MultiChoice / Aligned require live name from ctor sticky
	ClearErrorSess(testAmbientSession)
	if s := (&BooleanAttribute{Name: "", Prob: 100}).MakeRandom(NewRngSess(testAmbientSession, 1)); s != "" {
		t.Fatal("boolean empty name", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("boolean empty name must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if s := (&MultiChoiceAttribute{Name: "", Prob: 100, Choices: []string{"a"}}).MakeRandom(NewRngSess(testAmbientSession, 1)); s != "" {
		t.Fatal("multichoice empty name", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("multichoice empty name must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if s := (&AlignedAttribute{Name: "", Prob: 100, Alignment: 4}).MakeRandom(NewRngSess(testAmbientSession, 1)); s != "" {
		t.Fatal("aligned empty name", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("aligned empty name must SetError sticky")
	}
	// empty choice slot sticky — no invent visibility("")
	ClearErrorSess(testAmbientSession)
	if s := (&MultiChoiceAttribute{Name: "visibility", Prob: 100, Choices: []string{""}}).MakeRandom(NewRngSess(testAmbientSession, 1)); s != "" {
		t.Fatal("empty choice must fail closed", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty choice must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSectionAttribute(t *testing.T) {
	a := &SectionAttribute{Name: "section", Prob: 100}
	s := a.MakeRandom(NewRngSess(testAmbientSession, 3))
	if !strings.HasPrefix(s, "section(\"usersection") || !strings.HasSuffix(s, "\")") {
		t.Fatal(s)
	}
}

func TestAttributeNilRNGSticky(t *testing.T) {
	// Attribute / generator always have process RNG; sticky no invent skip shells
	ClearErrorSess(testAmbientSession)
	if (&BooleanAttribute{Name: "unused", Prob: 100}).MakeRandom(nil) != "" {
		t.Fatal("nil RNG boolean must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG BooleanAttribute must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&MultiChoiceAttribute{Name: "visibility", Prob: 100, Choices: []string{"default"}}).MakeRandom(nil) != "" {
		t.Fatal("nil RNG multichoice must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MultiChoiceAttribute must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&MultiChoiceAttribute{Name: "visibility", Prob: 100, Choices: nil}).MakeRandom(NewRngSess(testAmbientSession, 1)) != "" {
		t.Fatal("empty choices must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty choices MultiChoiceAttribute must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&AlignedAttribute{Name: "aligned", Prob: 100, Alignment: 4}).MakeRandom(nil) != "" {
		t.Fatal("nil RNG aligned must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG AlignedAttribute must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&SectionAttribute{Name: "section", Prob: 100}).MakeRandom(nil) != "" {
		t.Fatal("nil RNG section must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG SectionAttribute must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	g := &AttributeGenerator{Attributes: []Attribute{&BooleanAttribute{Name: "unused", Prob: 100}}}
	if g.Output(nil) != "" {
		t.Fatal("nil RNG generator Output must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG AttributeGenerator.Output must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
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
	out := st.OutputStructDeclOpts(NewRngSess(testAmbientSession, 1), attrs)
	if !strings.Contains(out, "struct S0") || !strings.Contains(out, "unused") {
		t.Fatal(out)
	}
}

func TestGetEvalToSubexpsComma(t *testing.T) {
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntType(), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntType(), false, false)
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
	// incomplete IR must IncompleteExpressions sticky (not bare nil invent empty-complete)
	ClearErrorSess(testAmbientSession)
	cases := []*Expression{
		{Term: TermCommaExpr},
		{Term: TermAssignment},
		{Term: TermVariable},
		{Term: TermFunction},
		{Term: TermConstant},
		nil,
	}
	for _, e := range cases {
		ClearErrorSess(testAmbientSession)
		if ExpressionsComplete(GetEvalToSubexps(e)) {
			t.Fatalf("incomplete eval must IncompleteExpressions, got complete for %#v", e)
		}
		if !HasErrorSess(testAmbientSession) {
			t.Fatalf("incomplete eval must SetError sticky for %#v", e)
		}
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil Constant shell sticky (no invent self-eval complete list)
	if ExpressionsComplete(GetEvalToSubexps(&Expression{
		Term: TermConstant, Con: &Constant{Value: "0"},
	})) {
		t.Fatal("Type-nil Constant must IncompleteExpressions")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil Constant GetEvalToSubexps must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil Variable shell sticky (specials exempt)
	if ExpressionsComplete(GetEvalToSubexps(&Expression{
		Term: TermVariable, Var: &Variable{Name: "g_hole", Type: nil},
	})) {
		t.Fatal("Type-nil Variable must IncompleteExpressions")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil Variable GetEvalToSubexps must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil LhsVar assign sticky
	if ExpressionsComplete(GetEvalToSubexps(&Expression{
		Term:   TermAssignment,
		Assign: &Stmt{Kind: StmtAssign, LhsVar: &Variable{Name: "g_hole", Type: nil}},
	})) {
		t.Fatal("Type-nil LhsVar must IncompleteExpressions")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil LhsVar GetEvalToSubexps must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestHaveOverlappingFieldsUnion(t *testing.T) {
	ut := &Type{isUnion: true, StructName: "U0", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	uv := CreateVariableQferSess(testAmbientSession, "g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	f0 := &Variable{Name: "g_u.f0", Type: GetIntType(), FieldVarOf: uv}
	f1 := &Variable{Name: "g_u.f1", Type: GetIntType(), FieldVarOf: uv}
	uv.FieldVars = []*Variable{f0, f1}
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), false, false)
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
	// fair: incomplete facts/pointees fail closed sticky as overlap
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), true, false)
	e1 := &Expression{Term: TermVariable, Var: p, ExprType: GetIntType()}
	e2 := &Expression{Term: TermVariable, Var: p, ExprType: GetIntType()}
	holeFacts := []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	if !HaveOverlappingFields(e1, e2, holeFacts) {
		t.Fatal("incomplete facts must fail closed as overlap")
	}
	if VariablesComplete(FindUnionPointees(holeFacts, e1)) {
		t.Fatal("FindUnionPointees incomplete must fail closed incomplete, not invent empty complete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("FindUnionPointees incomplete must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
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

func TestFindUnionPointeesGetContainerUnionResidualSticky(t *testing.T) {
	// GetContainerUnion residual soft invent was soft-continue later pointees invent empty unions.
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), false, false)
	// Type-nil parent ancestry: GetContainerUnion stickies ERROR
	parent := &Variable{Name: "g_hole"} // Type nil
	fld := &Variable{Name: "g_hole.f0", Type: GetIntType(), FieldVarOf: parent}
	facts := []*FactPointTo{MakeFactPointToSet(p, []*Variable{fld})}
	e := &Expression{Term: TermVariable, Var: p, ExprType: GetIntType()}
	got := FindUnionPointees(facts, e)
	if VariablesComplete(got) {
		t.Fatal("GetContainerUnion residual must fail closed incomplete, not invent empty complete", got)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GetContainerUnion residual FindUnionPointees must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// residual must also invent overlap (restrictive) not conflict-free
	if !HaveOverlappingFields(e, e, facts) {
		t.Fatal("GetContainerUnion residual HaveOverlappingFields must fail closed as overlap")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GetContainerUnion residual HaveOverlappingFields must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
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
	f.RV = CreateVariableScalarsSess(testAmbientSession, "f_rv", PointerTo(GetIntType()), false, false)
	loc := CreateVariableScalarsSess(testAmbientSession, "l_1", GetIntType(), false, false)
	blk := &Block{Func: f, LocalVars: []*Variable{loc}}
	f.Stack = []*Block{blk}
	lp := CreateVariableScalarsSess(testAmbientSession, "l_p", PointerTo(GetIntType()), false, false)
	fm := NewFactMgrSess(testAmbientSession, f)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(lp, loc)}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	st := Stmt{
		Kind: StmtReturn,
		Expr: &Expression{Term: TermVariable, Var: lp, ExprType: PointerTo(GetIntType())},
	}
	if VisitFactsStatementReturn(&st, &cg, opts) {
		t.Fatal("should reject local-pointing return")
	}
}

func TestVisitFactsLhsCompoundRead(t *testing.T) {
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntType(), false, false)
	lhs := &Lhs{Var: v, Type: GetIntType(), CompoundAssign: true}
	cg := EmptyCGContext().WithSession(testAmbientSession)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if !cg.VisitFactsLhs(lhs, Defaults()) {
		t.Fatal("compound")
	}
	// compound should have read then write
	if !eff.IsReadSess(testAmbientSession, v) || !eff.IsWrittenSess(testAmbientSession, v) {
		t.Fatal("rw", eff.IsReadSess(testAmbientSession, v), eff.IsWrittenSess(testAmbientSession, v))
	}
}

func TestAttributeGeneratorMakeRandomResidualSticky(t *testing.T) {
	// MakeRandom residual soft invent was soft-continue later attrs invent partial __attribute__.
	ClearErrorSess(testAmbientSession)
	g := &AttributeGenerator{Attributes: []Attribute{
		&BooleanAttribute{Name: "unused", Prob: 100},
		&BooleanAttribute{Name: "", Prob: 100}, // empty name residual sticky
	}}
	if s := g.Output(NewRngSess(testAmbientSession, 1)); s != "" {
		t.Fatal("MakeRandom residual must fail closed AttributeGenerator.Output", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("MakeRandom residual AttributeGenerator.Output must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
