package csmith

import "testing"

// FactMgr.cpp:268–270 — set_fact_out when s->parent == nullptr (function body Block)
// applies remove_function_local_facts: drop params as subjects + mark_func_end on
// remaining pointees (params as garbage). Nested blocks (parent != nil) do not.
// FunctionInvocationUser.cpp:212–221 — ret_facts = map_facts_out[body]; renew_facts.
func TestSetMapFactsOutForBlockFunctionBodyRemovesParams(t *testing.T) {
	ClearError()
	fn := &Function{Name: "func_x", ReturnType: GetIntType()}
	p := CreateVariableScalars("p_1", PointerTo(GetIntType()), false, false)
	if p == nil {
		t.Fatal("param")
	}
	fn.Param = []*Variable{p}
	g := CreateVariableScalars("g_1", PointerTo(GetIntType()), false, false)
	if g == nil {
		t.Fatal("global")
	}
	// g points at param — after function exit must become garbage
	body := &Block{Func: fn, Parent: nil, StmID: AllocStmID()}
	fn.Body = body
	fm := NewFactMgr(fn)
	facts := []*FactPointTo{
		MakeFactPointTo(p, NullPtr),
		MakeFactPointTo(g, p),
	}
	fm.SetMapFactsOutForBlock(body, facts)
	if HasError() {
		t.Fatalf("sticky: %v", HasError())
	}
	out := fm.GetMapFactsOut(body.StmID)
	if !FactsComplete(out) {
		t.Fatal("out incomplete", out)
	}
	// param subject dropped
	for _, f := range out {
		if f.Var == p || f.Var.Match(p) {
			t.Fatal("param subject must be removed from function-body map_facts_out", f)
		}
	}
	// global remaining; param pointee → garbage
	found := false
	for _, f := range out {
		if f.Var == g || f.Var.Match(g) {
			found = true
			hasG := false
			for _, pt := range f.PointTo {
				if pt == GarbagePtr {
					hasG = true
				}
				if pt == p {
					t.Fatal("param pointee must be mark_func_end garbage, not live param")
				}
			}
			if !hasG {
				t.Fatal("want garbage pointee after mark_func_end", f.PointTo)
			}
		}
	}
	if !found {
		t.Fatal("global fact must remain")
	}
	// Nested block: parent non-nil — no remove_function_local_facts
	ClearError()
	inner := &Block{Func: fn, Parent: body, StmID: AllocStmID()}
	facts2 := []*FactPointTo{
		MakeFactPointTo(p, NullPtr),
		MakeFactPointTo(g, p),
	}
	fm.SetMapFactsOutForBlock(inner, facts2)
	if HasError() {
		t.Fatalf("nested sticky: %v", HasError())
	}
	out2 := fm.GetMapFactsOut(inner.StmID)
	if len(out2) != 2 {
		t.Fatalf("nested block must store both facts, got %d", len(out2))
	}
	ClearError()
}

func TestSetMapFactsOutForBlockNilFailClosed(t *testing.T) {
	ClearError()
	(*FactMgr)(nil).SetMapFactsOutForBlock(&Block{StmID: 1}, nil)
	if !HasError() {
		t.Fatal("nil FM must sticky")
	}
	ClearError()
	fm := NewFactMgr(&Function{Name: "f"})
	fm.SetMapFactsOutForBlock(nil, []*FactPointTo{})
	if !HasError() {
		t.Fatal("nil Block must sticky")
	}
	ClearError()
}
