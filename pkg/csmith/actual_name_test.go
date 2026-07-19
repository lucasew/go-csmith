package csmith

import (
	"strings"
	"testing"
)

func TestGetActualNameAndPrefix(t *testing.T) {
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
}

func TestOutputDefVolatileComment(t *testing.T) {
	v := CreateVariableScalars("g_v", GetIntType(), false, true)
	v.Init = MakeInt(0)
	s := v.OutputDefOpts(true, false)
	if !strings.Contains(s, "VOLATILE GLOBAL g_v") {
		t.Fatal(s)
	}
	// local volatile — no comment
	l := CreateVariableScalars("l_v", GetIntType(), false, true)
	l.Init = MakeInt(1)
	s2 := l.OutputDefOpts(false, false)
	if strings.Contains(s2, "VOLATILE GLOBAL") {
		t.Fatal(s2)
	}
}

func TestOutputAddrOf(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	v.UseVolRVal = true
	// even with wrap, AddrOf uses bare name
	if v.OutputAddrOf(false) != "&g_1" {
		t.Fatal(v.OutputAddrOf(false))
	}
	// Variable.cpp:707 — always live Variable*; no invent "&0"
	if s := (*Variable)(nil).OutputAddrOf(false); s != "" {
		t.Fatal("nil must fail closed, got", s)
	}
	if s := (&Variable{}).OutputAddrOf(false); s != "" {
		t.Fatal("empty name must fail closed bare &, got", s)
	}
}

func TestOutputBoundNoInventFieldWithoutDot(t *testing.T) {
	// Variable.cpp:724–727 assert(dot != npos); no invent base-only field path
	parent := CreateVariableScalars("g_s", GetIntType(), false, false)
	f := &Variable{Name: "broken_field", Type: GetIntType(), FieldVarOf: parent}
	if s := f.OutputUpperBound(false); s != "" {
		t.Fatal("field without '.' must fail closed", s)
	}
	if s := f.OutputLowerBound(false); s != "" {
		t.Fatal("field without '.' must fail closed", s)
	}
	f.Name = "g_s.f0"
	if s := f.OutputUpperBound(false); s != "g_s.f0" {
		t.Fatal(s)
	}
}

func TestBlockNoInventIndentOnlyIncompleteStmt(t *testing.T) {
	// incomplete break must not invent whitespace-only indented line
	b := &Block{Stmts: []Stmt{{Kind: StmtBreak}}} // no Expr
	out := b.Output(1)
	// only braces / block shell, no stray indent lines that look like empty stmts
	if strings.Contains(out, "break") || strings.Contains(out, "if (") {
		t.Fatal(out)
	}
}

func TestNewProgramGeneratorSharesSessionProbs(t *testing.T) {
	// C++ Probabilities singleton — generator and VS must share one table
	opts := Defaults()
	g := NewProgramGenerator(opts)
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
	g := NewProgramGenerator(opts)
	// Force a known scalar volatile global through OutputGlobals path.
	v := CreateVariableScalars("g_v", GetIntType(), false, true)
	v.Init = MakeInt(0)
	g.VS.GlobalList = []*Variable{v}
	out := g.OutputGlobals()
	if !strings.Contains(out, "VOLATILE GLOBAL g_v") {
		t.Fatal(out)
	}
}

func TestOutputGlobalsVolatileArrayComment(t *testing.T) {
	opts := Defaults()
	g := NewProgramGenerator(opts)
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
	if !strings.Contains(out, "VOLATILE GLOBAL g_a") {
		t.Fatal(out)
	}
}

func TestOutputGlobalsIncompleteSticky(t *testing.T) {
	// incomplete GlobalList / Arrays fail closed sticky (no invent empty section)
	opts := Defaults()
	g := NewProgramGenerator(opts)
	v := CreateVariableScalars("g_v", GetIntType(), false, false)
	v.Init = MakeInt(0)
	g.VS.GlobalList = []*Variable{v, nil}
	ClearError()
	if g.OutputGlobals() != "" {
		t.Fatal("nil GlobalList hole must fail closed empty")
	}
	if !HasError() {
		t.Fatal("nil GlobalList hole must SetError sticky")
	}
	ClearError()
	g.VS.GlobalList = []*Variable{v}
	g.VS.Arrays = []*ArrayVariable{nil}
	if g.OutputGlobals() != "" {
		t.Fatal("nil Arrays hole must fail closed empty")
	}
	if !HasError() {
		t.Fatal("nil Arrays hole must SetError sticky")
	}
	ClearError()
}
