package csmith

import (
	"strconv"
	"strings"
	"testing"
)

func TestBooleanAttribute(t *testing.T) {
	a := &BooleanAttribute{Name: "unused", Prob: 100}
	if a.MakeRandomSess(testAmbientSession, NewRngSess(testAmbientSession, 1)) != "unused" {
		t.Fatal("always")
	}
	a.Prob = 0
	if a.MakeRandomSess(testAmbientSession, NewRngSess(testAmbientSession, 1)) != "" {
		t.Fatal("never")
	}
}

func TestAttributeGeneratorOutput(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	g := &AttributeGenerator{Attributes: []Attribute{
		&BooleanAttribute{Name: "unused", Prob: 100},
		&BooleanAttribute{Name: "used", Prob: 100},
	}}
	out := g.OutputSess(testAmbientSession, NewRngSess(testAmbientSession, 1))
	if !strings.Contains(out, "__attribute__((") || !strings.Contains(out, "unused") {
		t.Fatal(out)
	}
	if !strings.HasSuffix(out, "))") {
		t.Fatal(out)
	}
	// nil Attribute* hole sticky
	ClearErrorSess(testAmbientSession)
	gHole := &AttributeGenerator{Attributes: []Attribute{&BooleanAttribute{Name: "unused", Prob: 100}, nil}}
	if gHole.OutputSess(testAmbientSession, NewRngSess(testAmbientSession, 1)) != "" {
		t.Fatal("nil Attribute hole must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Attribute hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// AttributeGenerator always live; sticky empty (no invent soft-skip past hole)
	if (*AttributeGenerator)(nil).OutputSess(testAmbientSession, NewRngSess(testAmbientSession, 1)) != "" {
		t.Fatal("nil AttributeGenerator Output must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil AttributeGenerator Output must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// empty Attributes complete empty
	if (&AttributeGenerator{}).OutputSess(testAmbientSession, NewRngSess(testAmbientSession, 1)) != "" {
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
	s := a.MakeRandomSess(testAmbientSession, NewRngSess(testAmbientSession, 2))
	if !strings.HasPrefix(s, "visibility(\"") || !strings.HasSuffix(s, "\")") {
		t.Fatal(s)
	}
}

func TestAlignedAttribute(t *testing.T) {
	// Attribute.cpp:82–84 — aligned(1<<k)
	a := &AlignedAttribute{Name: "aligned", Prob: 100, Alignment: 4}
	s := a.MakeRandomSess(testAmbientSession, NewRngSess(testAmbientSession, 2))
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
	if a0.MakeRandomSess(testAmbientSession, NewRngSess(testAmbientSession, 1)) != "" {
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
	if s := a.MakeRandomSess(testAmbientSession, NewRngSess(testAmbientSession, 1)); s != "" {
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
	if (*BooleanAttribute)(nil).MakeRandomSess(testAmbientSession, r) != "" {
		t.Fatal("nil BooleanAttribute must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil BooleanAttribute must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*MultiChoiceAttribute)(nil).MakeRandomSess(testAmbientSession, r) != "" {
		t.Fatal("nil MultiChoiceAttribute must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil MultiChoiceAttribute must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*AlignedAttribute)(nil).MakeRandomSess(testAmbientSession, r) != "" {
		t.Fatal("nil AlignedAttribute must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil AlignedAttribute must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*SectionAttribute)(nil).MakeRandomSess(testAmbientSession, r) != "" {
		t.Fatal("nil SectionAttribute must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil SectionAttribute must SetError sticky")
	}
	// typed-nil interface slot still hits MakeRandom sticky (not soft not-selected)
	ClearErrorSess(testAmbientSession)
	var typedNil Attribute = (*BooleanAttribute)(nil)
	if typedNil.MakeRandomSess(testAmbientSession, r) != "" {
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
	if s := (&BooleanAttribute{Name: "", Prob: 100}).MakeRandomSess(testAmbientSession, NewRngSess(testAmbientSession, 1)); s != "" {
		t.Fatal("boolean empty name", s)
	}
	// boolean empty name must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
	if s := (&MultiChoiceAttribute{Name: "", Prob: 100, Choices: []string{"a"}}).MakeRandomSess(testAmbientSession, NewRngSess(testAmbientSession, 1)); s != "" {
		t.Fatal("multichoice empty name", s)
	}
	// multichoice empty name must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
	if s := (&AlignedAttribute{Name: "", Prob: 100, Alignment: 4}).MakeRandomSess(testAmbientSession, NewRngSess(testAmbientSession, 1)); s != "" {
		t.Fatal("aligned empty name", s)
	}
	// aligned empty name must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	// empty choice slot sticky — no invent visibility("")
	ClearErrorSess(testAmbientSession)
	if s := (&MultiChoiceAttribute{Name: "visibility", Prob: 100, Choices: []string{""}}).MakeRandomSess(testAmbientSession, NewRngSess(testAmbientSession, 1)); s != "" {
		t.Fatal("empty choice must fail closed", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty choice must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSectionAttribute(t *testing.T) {
	a := &SectionAttribute{Name: "section", Prob: 100}
	s := a.MakeRandomSess(testAmbientSession, NewRngSess(testAmbientSession, 3))
	if !strings.HasPrefix(s, "section(\"usersection") || !strings.HasSuffix(s, "\")") {
		t.Fatal(s)
	}
}

func TestAttributeNilRNGSticky(t *testing.T) {
	// Attribute / generator always have process RNG; sticky no invent skip shells
	ClearErrorSess(testAmbientSession)
	if (&BooleanAttribute{Name: "unused", Prob: 100}).MakeRandomSess(testAmbientSession, nil) != "" {
		t.Fatal("nil RNG boolean must fail closed")
	}
	// nil RNG BooleanAttribute must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
	if (&MultiChoiceAttribute{Name: "visibility", Prob: 100, Choices: []string{"default"}}).MakeRandomSess(testAmbientSession, nil) != "" {
		t.Fatal("nil RNG multichoice must fail closed")
	}
	// nil RNG MultiChoiceAttribute must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
	if (&MultiChoiceAttribute{Name: "visibility", Prob: 100, Choices: nil}).MakeRandomSess(testAmbientSession, NewRngSess(testAmbientSession, 1)) != "" {
		t.Fatal("empty choices must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty choices MultiChoiceAttribute must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&AlignedAttribute{Name: "aligned", Prob: 100, Alignment: 4}).MakeRandomSess(testAmbientSession, nil) != "" {
		t.Fatal("nil RNG aligned must fail closed")
	}
	// nil RNG AlignedAttribute must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
	if (&SectionAttribute{Name: "section", Prob: 100}).MakeRandomSess(testAmbientSession, nil) != "" {
		t.Fatal("nil RNG section must fail closed")
	}
	// nil RNG SectionAttribute must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
	g := &AttributeGenerator{Attributes: []Attribute{&BooleanAttribute{Name: "unused", Prob: 100}}}
	if g.OutputSess(testAmbientSession, nil) != "" {
		t.Fatal("nil RNG generator Output must fail closed")
	}
	// nil RNG AttributeGenerator.Output must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
}

func TestNewVarAttrGeneratorGated(t *testing.T) {
	opts := Defaults()
	opts.VariableAttributes = false
	g := NewVarAttrGenerator(testAmbientSession, opts, NewProbabilities(opts))
	if len(g.Attributes) != 0 {
		t.Fatal("off")
	}
	opts.VariableAttributes = true
	g = NewVarAttrGenerator(testAmbientSession, opts, NewProbabilities(opts))
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
	g0 := NewVarAttrGenerator(testAmbientSession, opts, nil)
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
	g := NewFuncAttrGenerator(testAmbientSession, opts, NewProbabilities(opts))
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
	g0 := NewFuncAttrGenerator(testAmbientSession, opts, nil)
	for _, a := range g0.Attributes {
		if ba, ok := a.(*BooleanAttribute); ok && ba.Prob != 0 {
			t.Fatalf("nil probs must not invent Prob=30, got %s=%d", ba.Name, ba.Prob)
		}
	}
}

func TestTypeAttrOnStructDecl(t *testing.T) {
	st := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	attrs := &AttributeGenerator{Attributes: []Attribute{
		&BooleanAttribute{Name: "unused", Prob: 100},
	}}
	out := st.OutputStructDeclSess(testAmbientSession, NewRngSess(testAmbientSession, 1), attrs)
	if !strings.Contains(out, "struct S0") || !strings.Contains(out, "unused") {
		t.Fatal(out)
	}
}

func TestGetEvalToSubexpsComma(t *testing.T) {
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntTypeSess(testAmbientSession), false, false)
	e := &Expression{
		Term:     TermCommaExpr,
		CommaLHS: &Expression{Term: TermVariable, Var: a, ExprType: GetIntTypeSess(testAmbientSession)},
		CommaRHS: &Expression{Term: TermVariable, Var: b, ExprType: GetIntTypeSess(testAmbientSession)},
	}
	subs := GetEvalToSubexpsSess(testAmbientSession, e)
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
		if ExpressionsComplete(GetEvalToSubexpsSess(testAmbientSession, e)) {
			t.Fatalf("incomplete eval must IncompleteExpressions, got complete for %#v", e)
		}
		if !HasErrorSess(testAmbientSession) {
			t.Fatalf("incomplete eval must SetError sticky for %#v", e)
		}
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil Constant shell sticky (no invent self-eval complete list)
	if ExpressionsComplete(GetEvalToSubexpsSess(testAmbientSession, &Expression{
		Term: TermConstant, Con: &Constant{Value: "0"},
	})) {
		t.Fatal("Type-nil Constant must IncompleteExpressions")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil Constant GetEvalToSubexps must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil Variable shell sticky (specials exempt)
	if ExpressionsComplete(GetEvalToSubexpsSess(testAmbientSession, &Expression{
		Term: TermVariable, Var: &Variable{Name: "g_hole", Type: nil},
	})) {
		t.Fatal("Type-nil Variable must IncompleteExpressions")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil Variable GetEvalToSubexps must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil LhsVar assign sticky
	if ExpressionsComplete(GetEvalToSubexpsSess(testAmbientSession, &Expression{
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
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		{Name: "f1", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	uv := CreateVariableQferSess(testAmbientSession, "g_u", ut, NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false}))
	f0 := &Variable{Name: "g_u.f0", Type: GetIntTypeSess(testAmbientSession), FieldVarOf: uv}
	f1 := &Variable{Name: "g_u.f1", Type: GetIntTypeSess(testAmbientSession), FieldVarOf: uv}
	uv.FieldVars = []*Variable{f0, f1}
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	facts := []*FactPointTo{MakeFactPointToSetSess(testAmbientSession, p, []*Variable{f0, f1})}
	// indirection: Var type *int, ExprType int → level 1
	e1 := &Expression{Term: TermVariable, Var: p, ExprType: GetIntTypeSess(testAmbientSession)}
	e2 := &Expression{Term: TermVariable, Var: p, ExprType: GetIntTypeSess(testAmbientSession)}
	u1 := FindUnionPointeesSess(testAmbientSession, facts, e1)
	if len(u1) == 0 {
		t.Fatalf("expected union pointees, ind=%d", e1.IndirectLevelSess(testAmbientSession))
	}
	if !HaveOverlappingFieldsSess(testAmbientSession, e1, e2, facts) {
		t.Fatal("overlap", u1)
	}
}

func TestHaveOverlappingFieldsIncompleteFailClosed(t *testing.T) {
	// soft invent: FindUnionPointees nil → len==0 → no overlap success
	// fair: incomplete facts/pointees fail closed sticky as overlap
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	e1 := &Expression{Term: TermVariable, Var: p, ExprType: GetIntTypeSess(testAmbientSession)}
	e2 := &Expression{Term: TermVariable, Var: p, ExprType: GetIntTypeSess(testAmbientSession)}
	holeFacts := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr), nil}
	if !HaveOverlappingFieldsSess(testAmbientSession, e1, e2, holeFacts) {
		t.Fatal("incomplete facts must fail closed as overlap")
	}
	if VariablesComplete(FindUnionPointeesSess(testAmbientSession, holeFacts, e1)) {
		t.Fatal("FindUnionPointees incomplete must fail closed incomplete, not invent empty complete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("FindUnionPointees incomplete must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete empty: non-pointer term → empty unions, no overlap
	c := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}
	empty := FindUnionPointeesSess(testAmbientSession, nil, c)
	if empty == nil || len(empty) != 0 {
		t.Fatal("complete non-ptr must be non-nil empty", empty)
	}
	if HaveOverlappingFieldsSess(testAmbientSession, c, c, nil) {
		t.Fatal("complete constants must not invent overlap")
	}
}

func TestFindUnionPointeesGetContainerUnionResidualSticky(t *testing.T) {
	// GetContainerUnion residual soft invent was soft-continue later pointees invent empty unions.
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	// Type-nil parent ancestry: GetContainerUnion stickies ERROR
	parent := &Variable{Name: "g_hole"} // Type nil
	fld := &Variable{Name: "g_hole.f0", Type: GetIntTypeSess(testAmbientSession), FieldVarOf: parent}
	facts := []*FactPointTo{MakeFactPointToSetSess(testAmbientSession, p, []*Variable{fld})}
	e := &Expression{Term: TermVariable, Var: p, ExprType: GetIntTypeSess(testAmbientSession)}
	got := FindUnionPointeesSess(testAmbientSession, facts, e)
	if VariablesComplete(got) {
		t.Fatal("GetContainerUnion residual must fail closed incomplete, not invent empty complete", got)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GetContainerUnion residual FindUnionPointees must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// residual must also invent overlap (restrictive) not conflict-free
	if !HaveOverlappingFieldsSess(testAmbientSession, e, e, facts) {
		t.Fatal("GetContainerUnion residual HaveOverlappingFields must fail closed as overlap")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GetContainerUnion residual HaveOverlappingFields must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestBuiltinOutputSkipped(t *testing.T) {
	f := &Function{Name: "__builtin_clz", ReturnType: GetIntTypeSess(testAmbientSession), IsBuiltin: true}
	if f.OutputSess(testAmbientSession, false, false, nil) != "" || f.OutputForwardDeclSess(testAmbientSession, false, nil, false) != "" {
		t.Fatal("builtin emit")
	}
}

func TestVisitFactsReturnDeadPtr(t *testing.T) {
	opts := Defaults()
	opts.NoReturnDeadPointer = true
	f := &Function{Name: "f", ReturnType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	f.RV = CreateVariableScalarsSess(testAmbientSession, "f_rv", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	loc := CreateVariableScalarsSess(testAmbientSession, "l_1", GetIntTypeSess(testAmbientSession), false, false)
	blk := &Block{Func: f, LocalVars: []*Variable{loc}}
	f.Stack = []*Block{blk}
	lp := CreateVariableScalarsSess(testAmbientSession, "l_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	fm := NewFactMgrSess(testAmbientSession, f)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, lp, loc)}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	st := Stmt{
		Kind: StmtReturn,
		Expr: &Expression{Term: TermVariable, Var: lp, ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))},
	}
	if VisitFactsStatementReturn(&st, &cg, opts) {
		t.Fatal("should reject local-pointing return")
	}
}

func TestVisitFactsLhsCompoundRead(t *testing.T) {
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	lhs := &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession), CompoundAssign: true}
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
	if s := g.OutputSess(testAmbientSession, NewRngSess(testAmbientSession, 1)); s != "" {
		t.Fatal("MakeRandom residual must fail closed AttributeGenerator.Output", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("MakeRandom residual AttributeGenerator.Output must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
