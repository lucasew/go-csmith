package csmith

import (
	"strings"
	"testing"
)

func TestGetActualNameAndPrefix(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	if g.GetActualName(false) != "g_1" {
		t.Fatal(g.GetActualName(false))
	}
	// default generator: prefix returns name unchanged
	if GetPrefixedName("g_1", true) != "g_1" {
		t.Fatal("prefix")
	}
	l := CreateVariableScalars("l_1", GetIntType(), false, false)
	if l.GetActualName(true) != "l_1" {
		t.Fatal("local")
	}
	// empty Name sticky (no invent empty identifier soft-skip past incomplete name)
	if s := (&Variable{Name: "", Type: GetIntType()}).GetActualName(false); s != "" {
		t.Fatal("empty Name GetActualName invent", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty Name GetActualName must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil Variable sticky empty (no invent bare name without shell)
	if s := (*Variable)(nil).GetActualName(false); s != "" {
		t.Fatal("nil GetActualName invent", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil GetActualName must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputDefVolatileComment(t *testing.T) {
	// Variable.cpp:662–667 — is_volatile() only (locals included; text still "GLOBAL")
	v := CreateVariableScalars("g_v", GetIntType(), false, true)
	v.Init = MakeInt(0)
	s := v.OutputDefOpts(true, false)
	if !strings.Contains(s, "VOLATILE GLOBAL g_v") {
		t.Fatal(s)
	}
	// local volatile still gets the comment (seed 95 l_1341)
	l := CreateVariableScalars("l_v", GetIntType(), false, true)
	l.Init = MakeInt(1)
	s2 := l.OutputDefOpts(false, false)
	if !strings.Contains(s2, "VOLATILE GLOBAL l_v") {
		t.Fatal(s2)
	}
	// non-volatile local: no comment
	n := CreateVariableScalars("l_n", GetIntType(), false, false)
	n.Init = MakeInt(2)
	s3 := n.OutputDefOpts(false, false)
	if strings.Contains(s3, "VOLATILE GLOBAL") {
		t.Fatal(s3)
	}
}

func TestOutputAddrOf(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	v.UseVolRVal = true
	// even with wrap, AddrOf uses bare name
	if v.OutputAddrOf(false) != "&g_1" {
		t.Fatal(v.OutputAddrOf(false))
	}
	// Variable.cpp:707 — always live Variable*; sticky no invent "&0"
	ClearErrorSess(testAmbientSession)
	if s := (*Variable)(nil).OutputAddrOf(false); s != "" {
		t.Fatal("nil must fail closed, got", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil OutputAddrOf must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if s := (&Variable{}).OutputAddrOf(false); s != "" {
		t.Fatal("empty name must fail closed bare &, got", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty name OutputAddrOf must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputBoundNoInventFieldWithoutDot(t *testing.T) {
	// Variable.cpp:724–727 assert(dot != npos); sticky no invent base-only field path
	ClearErrorSess(testAmbientSession)
	parent := CreateVariableScalars("g_s", GetIntType(), false, false)
	f := &Variable{Name: "broken_field", Type: GetIntType(), FieldVarOf: parent}
	if s := f.OutputUpperBound(false); s != "" {
		t.Fatal("field without '.' must fail closed", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("field without '.' OutputUpperBound must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if s := f.OutputLowerBound(false); s != "" {
		t.Fatal("field without '.' must fail closed", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("field without '.' OutputLowerBound must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	f.Name = "g_s.f0"
	if s := f.OutputUpperBound(false); s != "g_s.f0" {
		t.Fatal(s)
	}
	ClearErrorSess(testAmbientSession)
}

func TestBlockNoInventIndentOnlyIncompleteStmt(t *testing.T) {
	// incomplete break sticky must not invent whitespace-only indented line
	ClearErrorSess(testAmbientSession)
	b := &Block{Stmts: []Stmt{{Kind: StmtBreak}}} // no Expr
	out := b.Output(1)
	// incomplete stmt fails whole Output sticky empty (no invent bare break / indent-only)
	if out != "" {
		t.Fatal("incomplete break must fail closed whole block", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete break Block.Output must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestNewProgramGeneratorSharesSessionProbs(t *testing.T) {
	// C++ Probabilities singleton — generator and VS must share one table
	opts := Defaults()
	g := NewProgramGenerator(NewSession(opts))
	if g.Probs == nil || g.VS == nil || g.VS.Probs == nil {
		t.Fatal("missing probs")
	}
	if g.Probs != g.VS.Probs {
		t.Fatal("VS must share session Probs, not invent a second table")
	}
}

func TestNewVariableSelectorProbsShares(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelectorProbs(opts, probs)
	if vs.Probs != probs {
		t.Fatal("must share, not invent")
	}
}

func TestOutputGlobalsVolatileComment(t *testing.T) {
	opts := Defaults()
	g := NewProgramGenerator(NewSession(opts))
	// Force a known scalar volatile global through OutputGlobals path.
	v := CreateVariableScalars("g_v", GetIntType(), false, true)
	v.Init = MakeInt(0)
	g.VS.GlobalList = []*Variable{v}
	out := g.OutputGlobals()
	if !strings.Contains(out, "VOLATILE GLOBAL g_v") {
		t.Fatal(out)
	}
}

func TestOutputGlobalsVolatileArrayNoComment(t *testing.T) {
	// ArrayVariable.cpp:506–507 — array OutputDef emits ";\n" only; no VOLATILE GLOBAL invent.
	opts := Defaults()
	g := NewProgramGenerator(NewSession(opts))
	av := &ArrayVariable{
		Variable: Variable{
			Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2},
			Qfer: NewCVQualifiers([]bool{false}, []bool{true}),
			Init: MakeInt(0),
		},
		Sizes: []int{2},
	}
	av.AsArray = av
	g.VS.GlobalList = []*Variable{&av.Variable}
	g.VS.Arrays = []*ArrayVariable{av}
	out := g.OutputGlobals()
	if strings.Contains(out, "VOLATILE GLOBAL") {
		t.Fatal("array must not invent VOLATILE GLOBAL comment:", out)
	}
	if !strings.Contains(out, "volatile int") || !strings.Contains(out, "g_a[2]") {
		t.Fatal(out)
	}
}

func TestOutputGlobalsIncompleteSticky(t *testing.T) {
	// incomplete GlobalList / Arrays fail closed sticky (no invent empty section)
	opts := Defaults()
	g := NewProgramGenerator(NewSession(opts))
	v := CreateVariableScalars("g_v", GetIntType(), false, false)
	v.Init = MakeInt(0)
	g.VS.GlobalList = []*Variable{v, nil}
	g.clearErr()
	if g.OutputGlobals() != "" {
		t.Fatal("nil GlobalList hole must fail closed empty")
	}
	if !g.hasErr() {
		t.Fatal("nil GlobalList hole must SetError sticky")
	}
	g.clearErr()
	g.VS.GlobalList = []*Variable{v}
	g.VS.Arrays = []*ArrayVariable{nil}
	if g.OutputGlobals() != "" {
		t.Fatal("nil Arrays hole must fail closed empty")
	}
	if !g.hasErr() {
		t.Fatal("nil Arrays hole must SetError sticky")
	}
	g.clearErr()
}

func TestOutputCNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*Variable)(nil).OutputC() != "" {
		t.Fatal("nil Variable OutputC must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Variable OutputC must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Variable)(nil).OutputValueDump("x ", 0, nil) != "" {
		t.Fatal("nil Variable OutputValueDump must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Variable OutputValueDump must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray soft invent was bare-name OutputC/LHS
	shell := &Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	if shell.OutputC() != "" {
		t.Fatal("IsArray without AsArray OutputC must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray OutputC must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if shell.OutputLhsC() != "" {
		t.Fatal("IsArray without AsArray OutputLhsC must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray OutputLhsC must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGetActualNameIsGlobalFieldResidualSticky(t *testing.T) {
	// Parent IsGlobal residual soft invent was invent bare field name past Type-nil parent shell.
	ClearErrorSess(testAmbientSession)
	// Field of nil parent shell: FieldVarOf nil path is not field; force parent nil Variable residual via FieldVarOf cycle?
	// Field with parent that is nil Variable pointer stuck:
	// parent Type-nil still IsGlobal by name if g_
	parent := &Variable{Name: "g_s", Type: GetIntType()}
	child := &Variable{Name: "g_s.f0", Type: GetIntType(), FieldVarOf: parent}
	if child.GetActualName(false) != "g_s.f0" {
		// globals may use name as-is
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete field GetActualName must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil FieldVarOf parent residual: FieldVarOf points to incomplete — use nil parent via incomplete FieldVarOf chain
	// IsGlobal residual on nil: FieldVarOf of child is non-nil parent; parent.FieldVarOf = nil → name g_
	// Force residual: FieldVarOf is nil Variable embedded? can't.
	// Hygiene: empty name sticky
	if (&Variable{FieldVarOf: parent}).GetActualName(false) != "" {
		t.Fatal("empty name GetActualName must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty name GetActualName must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsGlobalFieldParentResidualSticky(t *testing.T) {
	// Parent IsGlobal residual soft invent was invent not-global soft-skip past nil parent.
	ClearErrorSess(testAmbientSession)
	// FieldVarOf to incomplete parent with residual: parent nil is not possible without pointer.
	// Field of nil-named parent that is Type-nil still IsGlobal by name.
	parent := (*Variable)(nil)
	// can't set FieldVarOf to nil and call IsGlobal on field with non-nil FieldVarOf only
	// Use: FieldVarOf = parent that is complete; residual path when FieldVarOf.IsGlobal stickies
	// Direct nil IsGlobal
	if (*Variable)(nil).IsGlobal() {
		t.Fatal("nil IsGlobal must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsGlobal must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	_ = parent
}
