package csmith

import (
	"testing"
)

func TestGenerateBodyWithKnownParamsSetsRW(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 1
	opts.MaxBlockDepth = 1
	vs := NewVariableSelector(opts)
	g := vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), GetIntType(), nil, NewRng(1))
	// caller context with a no-write on global
	caller := &Function{Name: "func_1"}
	cblk := &Block{Func: caller}
	caller.Stack = []*Block{cblk}
	fm := NewFactMgr(caller)
	if g != nil {
		fm.AddNewVarFact(g)
	}
	accum := EmptyEffect()
	prev := WithFunc(caller, EmptyEffect()).WithFactMgr(fm)
	prev.EffectAccum = &accum
	prev.RW = &RWDirective{NoWriteVars: []*Variable{g}}

	callee := &Function{
		Name:       "func_2",
		ReturnType: GetIntType(),
		Param:      []*Variable{CreateVariableScalars("p_1", GetIntType(), false, false)},
	}
	callee.RV = CreateVariableQfer("func_2_rv", GetIntType(), NewCVQualifiers([]bool{false}, []bool{false}))
	// handover facts empty for new calFM inside generate
	callee.GenerateBodyWithKnownParams(NewRng(2), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), prev)
	if callee.BuildState != BuildBuilt {
		t.Fatal(callee.BuildState)
	}
	if callee.Body == nil {
		t.Fatal("no body")
	}
	// set_depth_protect(true) on body
	if !callee.Body.EmitDepthProtect {
		t.Fatal("want EmitDepthProtect on body")
	}
}

func TestGenerateBodyBuiltinDummy(t *testing.T) {
	opts := Defaults()
	f := &Function{
		Name:       "__builtin_clz",
		ReturnType: GetIntType(),
		IsBuiltin:  true,
	}
	f.GenerateBody(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), NewStatementThresholdTable(opts), EmptyCGContext())
	if f.Body == nil {
		t.Fatal("dummy body")
	}
	if f.BuildState != BuildBuilt {
		t.Fatal(f.BuildState)
	}
}

func TestMakeReturnConst(t *testing.T) {
	opts := Defaults()
	opts.DepthProtect = true
	f := &Function{Name: "f", ReturnType: GetIntType()}
	f.MakeReturnConst(opts, NewRng(1))
	if f.RetConst == nil {
		t.Fatal("want ret const")
	}
	// void — no
	f2 := &Function{Name: "v", ReturnType: GetSimpleType(EVoid)}
	f2.MakeReturnConst(opts, NewRng(1))
	if f2.RetConst != nil {
		t.Fatal("void no const")
	}
}

func TestMakeExpressionCommaNilLHSType(t *testing.T) {
	// ExpressionComma lhs type nullptr → choose_random_nonvoid needs Type env
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	seedTypesForTest(NewRng(1), opts, probs, vs, nil)
	_ = vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), GetIntType(), nil, NewRng(1))
	e := func() *Expression {
		c := EmptyCGContext()
		c.Types = vs.Types
		return MakeExpressionComma(NewRng(3), opts, probs, vs, NewExprTables(opts), &c, GetIntType(), nil)
	}()
	if e == nil || e.Term != TermCommaExpr {
		t.Fatal(e)
	}
	if e.CommaLHS == nil || e.CommaRHS == nil {
		t.Fatal("sides")
	}
}
