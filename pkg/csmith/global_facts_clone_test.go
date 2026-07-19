package csmith

import "testing"

func TestMakeRandomForIncompleteGlobalFactsFailClosed(t *testing.T) {
	// StatementFor.cpp:299–300 pre_facts snapshot; incomplete must not invent cleaned
	ClearError()
	opts := Defaults()
	opts.MaxBlockSize = 1
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	vs := NewVariableSelector(opts)
	if MakeRandomFor(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomFor")
	}
	ClearError()
}

func TestAppendReturnIncompleteGlobalFactsFailClosed(t *testing.T) {
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntType()}
	f.RV = CreateVariableScalars("f_rv", GetIntType(), false, false)
	b := &Block{StmID: 1, Func: f}
	fm := NewFactMgr(f)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	vs := NewVariableSelector(opts)
	if b.AppendReturnStmt(NewRng(2), opts, vs, &cg) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed AppendReturnStmt")
	}
}

func TestVisitFactsBinaryOrderedIncompleteGlobalFactsFailClosed(t *testing.T) {
	// after-left snapshot incomplete fails closed (no invent cleaned merge)
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	fi := &Invocation{
		Args: []*Expression{
			{Term: TermConstant, Con: MakeInt(1)},
			{Term: TermConstant, Con: MakeInt(0)},
		},
		Binary: "&&",
	}
	if VisitFactsBinaryOrdered(fi, &cg, Defaults()) {
		t.Fatal("incomplete GlobalFacts must fail closed ordered binary visit")
	}
}

func TestMakeRandomIfFunc1IncompleteGlobalFactsFailClosed(t *testing.T) {
	// func_1 pre_facts snapshot must fail closed on incomplete GlobalFacts
	ClearError()
	opts := Defaults()
	opts.MaxBlockSize = 1
	opts.MaxBlockDepth = 1
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	vs := NewVariableSelector(opts)
	// seed a global for expression selection
	_ = vs.GenerateNewGlobal(AccessRead, cg, GetIntType(), nil, NewRng(3))
	if MakeRandomIf(NewRng(4), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomIf func_1 path")
	}
	ClearError()
}
