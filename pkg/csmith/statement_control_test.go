package csmith

import (
	"strings"
	"testing"
)

func TestStmtReturnMustReturn(t *testing.T) {
	if !(Stmt{Kind: StmtReturn}).MustReturn() {
		t.Fatal("return must_return")
	}
	if (Stmt{Kind: StmtAssign}).MustReturn() {
		t.Fatal("assign")
	}
}

func TestIfMustReturnBothBranches(t *testing.T) {
	st := Stmt{
		Kind: StmtIfElse,
		Then: &Block{Stmts: []Stmt{{Kind: StmtReturn}}},
		Else: &Block{Stmts: []Stmt{{Kind: StmtReturn}}},
	}
	if !st.MustReturn() {
		t.Fatal("both return")
	}
	st.Else = &Block{Stmts: []Stmt{{Kind: StmtAssign}}}
	if st.MustReturn() {
		t.Fatal("only then")
	}
}

func TestBlockMustReturnLast(t *testing.T) {
	b := &Block{Stmts: []Stmt{{Kind: StmtAssign}, {Kind: StmtReturn}}}
	if !b.MustReturn() {
		t.Fatal("last return")
	}
	b2 := &Block{Stmts: []Stmt{{Kind: StmtReturn}, {Kind: StmtLabel, SourceLabel: "x"}}}
	// trailing label after return — still last non-label is return
	// wait order is return then label — MustReturn skips labels from end, finds return
	if !b2.MustReturn() {
		t.Fatal("label after return")
	}
}

func TestNeedReturnStmt(t *testing.T) {
	f := &Function{ReturnType: GetIntType()}
	if !f.NeedReturnStmt() {
		t.Fatal("int")
	}
	f.ReturnType = GetSimpleType(EVoid)
	if f.NeedReturnStmt() {
		t.Fatal("void")
	}
}

func TestGenerateFunctionsHaveReturnWhenNeeded(t *testing.T) {
	// non-void funcs should contain "return" in body
	for seed := uint64(1); seed < 30; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		// crude: every func_N body that is not void should have return —
		// at least func_1 with non-void almost always
		if strings.Contains(out, "int32_t func_1") || strings.Contains(out, "uint32_t func_1") ||
			strings.Contains(out, "int func_1") || strings.Contains(out, "int8_t func_1") ||
			strings.Contains(out, "int16_t func_1") || strings.Contains(out, "int64_t func_1") ||
			strings.Contains(out, "uint") && strings.Contains(out, "func_1") {
			if !strings.Contains(out, "return ") {
				t.Fatalf("seed %d non-void program without return", seed)
			}
			return
		}
	}
}

func TestHashNoEmptyArrayLoops(t *testing.T) {
	// pointer array should produce empty hash (skipped)
	v := &Variable{
		Name: "g_p", Type: PointerTo(GetIntType()), IsArray: true, ArraySizes: []int{4},
		Qfer: NewCVQualifiers([]bool{false}, []bool{false}),
	}
	if v.HashOutput() != "" {
		t.Fatal("pointer array must not hash")
	}
}

func TestHashGlobalVarsSharedIndices(t *testing.T) {
	vs := NewVariableSelector(Defaults())
	vs.GlobalList = []*Variable{
		{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2, 3}, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		{Name: "g_b", Type: GetIntType(), IsArray: true, ArraySizes: []int{4}, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
	}
	out := HashGlobalVariables(vs)
	// only one "int i0" decl
	if strings.Count(out, "int i0;") != 1 {
		t.Fatal(out)
	}
	if strings.Count(out, "int i1;") != 1 {
		t.Fatal(out)
	}
	if !strings.Contains(out, "g_a[i0][i1]") || !strings.Contains(out, "g_b[i0]") {
		t.Fatal(out)
	}
}
