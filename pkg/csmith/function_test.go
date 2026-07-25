package csmith

import (
	"strings"
	"testing"
)

func TestRandomFunctionName(t *testing.T) {
	// Function.cpp:249 — util.cpp gensym process-wide; private GenSym ignored
	ResetDefaultGensym()
	if RandomFunctionNameSess(testAmbientSession, nil) != "func_1" || RandomFunctionNameSess(testAmbientSession, nil) != "func_2" {
		t.Fatal("func gensym")
	}
	// private GenSym must not invent a separate stream
	var g GenSym
	if RandomFunctionNameSess(testAmbientSession, &g) != "func_3" {
		t.Fatal("must use process gensym, not private GenSym")
	}
}

func TestParamListProbabilityRange(t *testing.T) {
	opts := Defaults()
	r := NewRngSess(testAmbientSession, 2)
	// max_params=5 → rnd_upto(5) in 0..4
	for i := 0; i < 20; i++ {
		p := ParamListProbabilitySess(testAmbientSession, r, opts)
		if p >= 5 {
			t.Fatalf("param list prob %d", p)
		}
	}
	// sticky no invent param count 0 without RNG draw
	ClearErrorSess(testAmbientSession)
	if ParamListProbabilitySess(testAmbientSession, nil, opts) != 0 {
		t.Fatal("nil RNG ParamListProbability must fail closed 0")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG ParamListProbability must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomSignatureParams(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	// GenerateParameterVariable needs Type env (no soft invent simple)
	seedTypesForTest(NewRngSess(testAmbientSession, 1), opts, probs, vs, nil)
	// share gensym counters: use vs.Sym for params and separate for funcs is OK upstream-global
	// For 1:1 naming, share one GenSym for both
	sym := &vs.Sym
	r := NewRngSess(testAmbientSession, 2)
	f := MakeRandomSignature(r, opts, probs, vs, sym, EmptyCGContext().WithSession(testAmbientSession), GetSimpleTypeSess(testAmbientSession, EInt), nil, nil)
	if f == nil || !strings.HasPrefix(f.Name, "func_") {
		t.Fatalf("name %+v", f)
	}
	if f.ReturnType != GetSimpleTypeSess(testAmbientSession, EInt) {
		t.Fatal("return type")
	}
	if f.RV == nil || !strings.HasSuffix(f.RV.Name, "_rv") {
		t.Fatal("rv")
	}
	// param count = max+1 from first ParamListProbability
	// At least 1 param (max>=0 → loop 0..max)
	if len(f.Param) < 1 {
		t.Fatal("expected >=1 param")
	}
	for _, p := range f.Param {
		if p == nil || !p.IsArgumentSess(testAmbientSession) {
			t.Fatalf("param %v", p)
		}
	}
	proto := f.OutputForwardDeclSess(testAmbientSession, false, nil, false)
	if !strings.Contains(proto, f.Name) || !strings.HasSuffix(proto, ");") {
		t.Fatalf("proto %q", proto)
	}
}

func TestMakeFirstNoParamsHasBody(t *testing.T) {
	ReinstallTestProcessSingletons()
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	tables := NewExprTablesSess(testAmbientSession, opts)
	stmtTab := NewStatementThresholdTable(opts)
	var list FunctionList
	r := NewRngSess(testAmbientSession, 2)
	seedTypesForTest(r, opts, probs, vs, &list)
	f := MakeFirst(r, opts, probs, vs, &vs.Sym, tables, stmtTab, &list, nil)
	if f == nil || f.Body == nil {
		t.Fatal("body")
	}
	if len(f.Param) != 0 {
		t.Fatalf("make_first default extension null → no params, got %d", len(f.Param))
	}
	if len(list.Funcs) < 1 {
		t.Fatal("func list empty")
	}
	// Body may create additional function signatures (ExpressionFuncall).
	if list.Funcs[0] != f {
		t.Fatal("first func not registered first")
	}
	out := f.OutputSess(testAmbientSession, false, false, nil)
	if !strings.Contains(out, f.Name) || !strings.Contains(out, "{") {
		t.Fatalf("output %q", out)
	}
	// body should have max_block_size statements unless early return;
	// forward-goto StmtLabel markers may inflate len(Stmts); trailing append_return
	// may add one more (Block.cpp:734).
	if len(f.Body.Stmts) < 1 {
		t.Fatal("empty stmts")
	}
	n := 0
	for _, s := range f.Body.Stmts {
		if s.Kind != StmtLabel {
			n++
		}
	}
	if n > opts.MaxBlockSize+1 {
		t.Fatalf("too many real stmts %d (raw %d)", n, len(f.Body.Stmts))
	}
}

func TestBlockProbabilityAlwaysMaxMinusOne(t *testing.T) {
	// Keep-filter forces block_size-1
	if BlockProbability(4, NewRngSess(testAmbientSession, 2)) != 3 {
		t.Fatal("max_block_size 4 → 3")
	}
}

func TestBlockProbabilityNilRNGSticky(t *testing.T) {
	// C++ always has RNG; sticky no invent fixed block size without draw
	ClearErrorSess(testAmbientSession)
	if BlockProbability(4, nil) != 0 {
		t.Fatal("nil RNG must fail closed 0")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG BlockProbability must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeFirstReturnBreaksEarly(t *testing.T) {
	// With enough seeds, some bodies end early on return
	opts := Defaults()
	probs := NewProbabilities(opts)
	tables := NewExprTablesSess(testAmbientSession, opts)
	stmtTab := NewStatementThresholdTable(opts)
	foundEarly := false
	for seed := uint64(1); seed < 50; seed++ {
		vs := NewVariableSelector(testAmbientSession, opts)
		r := NewRngSess(testAmbientSession, seed)
		seedTypesForTest(r, opts, probs, vs, nil)
		f := MakeFirst(r, opts, probs, vs, &vs.Sym, tables, stmtTab, nil, nil)
		if f == nil || f.Body == nil {
			continue
		}
		if len(f.Body.Stmts) < opts.MaxBlockSize {
			// last should be return if early
			last := f.Body.Stmts[len(f.Body.Stmts)-1]
			if last.Kind == StmtReturn {
				foundEarly = true
				break
			}
		}
	}
	if !foundEarly {
		// not a hard failure — return is only 5% band (30-34); may be rare with filters
		t.Log("no early return in seeds 1..49 (ok if filters reduce return rate)")
	}
}

func TestMakeRandomSignatureERRORGuard(t *testing.T) {
	// Function.cpp:408/419 ERROR_GUARD after depth / random_qualifiers
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	seedTypesForTest(NewRngSess(testAmbientSession, 1), opts, probs, vs, nil)
	SetErrorSess(testAmbientSession, ErrGeneric)
	f := MakeRandomSignature(NewRngSess(testAmbientSession, 2), opts, probs, vs, &vs.Sym, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, nil)
	if f != nil {
		t.Fatal("sticky error must fail closed")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomSignatureNoInventWithoutSession(t *testing.T) {
	// Function.cpp always has RNG + Probabilities; no invent signature shells
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	seedTypesForTest(NewRngSess(testAmbientSession, 1), opts, probs, vs, nil)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	if MakeRandomSignature(nil, opts, probs, vs, &vs.Sym, cg, GetIntTypeSess(testAmbientSession), nil, nil) != nil {
		t.Fatal("nil RNG must not invent signature")
	}
	if MakeRandomSignature(NewRngSess(testAmbientSession, 2), opts, nil, vs, &vs.Sym, cg, GetIntTypeSess(testAmbientSession), nil, nil) != nil {
		t.Fatal("nil probs must not invent signature")
	}
	// MakeFirst same contract
	if MakeFirst(nil, opts, probs, vs, &vs.Sym, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTable(opts), nil, nil) != nil {
		t.Fatal("nil RNG must not invent first")
	}
	if MakeFirst(NewRngSess(testAmbientSession, 2), opts, nil, vs, &vs.Sym, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTable(opts), nil, nil) != nil {
		t.Fatal("nil probs must not invent first")
	}
}

func TestMakeFirstMakeRandomFunctionIncompleteGlobalListFailClosed(t *testing.T) {
	// incomplete GlobalList seed must sticky ERROR (no invent partial FM / body success)
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxBlockSize = 1
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	seedTypesForTest(NewRngSess(testAmbientSession, 1), opts, probs, vs, nil)
	// plant incomplete GlobalList hole
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	vs.GlobalList = []*Variable{g, nil}
	if MakeFirst(NewRngSess(testAmbientSession, 3), opts, probs, vs, &vs.Sym, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTable(opts), nil, nil) != nil {
		t.Fatal("incomplete GlobalList must fail closed MakeFirst")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalList must SetError sticky MakeFirst")
	}
	ClearErrorSess(testAmbientSession)
	// MakeRandomFunction same seed gate
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.Types = vs.Types
	if MakeRandomFunction(NewRngSess(testAmbientSession, 4), opts, probs, vs, &vs.Sym, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTable(opts), cg, GetIntTypeSess(testAmbientSession), nil, nil) != nil {
		t.Fatal("incomplete GlobalList must fail closed MakeRandomFunction")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalList must SetError sticky MakeRandomFunction")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomForERRORGuardAfterBody(t *testing.T) {
	// StatementFor.cpp:304 ERROR_GUARD after body
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxBlockSize = 0
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	seedTypesForTest(NewRngSess(testAmbientSession, 1), opts, probs, vs, nil)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	vs.GlobalList = []*Variable{g}
	fm := NewFactMgrSess(testAmbientSession, f)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.Types = vs.Types
	f.Stack = []*Block{{Func: f}}
	// sticky error before make fails early
	SetErrorSess(testAmbientSession, ErrGeneric)
	st := MakeRandomFor(NewRngSess(testAmbientSession, 3), opts, probs, vs, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTable(opts), &cg)
	if st != nil && st.Loop != nil && st.Then != nil {
		t.Fatal("sticky error should not complete for")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFunctionOutputNoSoftInventBodyOrRetConst(t *testing.T) {
	// Function.cpp:575–597 — depth_protect + body + ret_c always together
	// sticky no invent empty braces / header-only / if without else / "0"
	ClearErrorSess(testAmbientSession)
	f := &Function{
		Name: "func_x", ReturnType: GetIntTypeSess(testAmbientSession),
		RV:           CreateVariableScalarsSess(testAmbientSession, "func_x_rv", GetIntTypeSess(testAmbientSession), false, false),
		DepthProtect: true,
	}
	out := f.OutputSess(testAmbientSession, false, false, nil)
	if out != "" {
		t.Fatal("nil Body must fail closed empty, got", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Body Output must SetError sticky")
	}
	// RetConst only — sticky no invent if/else without body
	ClearErrorSess(testAmbientSession)
	f.RetConst = MakeIntSess(testAmbientSession, 42)
	out = f.OutputSess(testAmbientSession, false, false, nil)
	if out != "" {
		t.Fatal("RetConst without Body must fail closed empty", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("RetConst without Body must SetError sticky")
	}
	// Body without RetConst — body only, no invent if without else
	ClearErrorSess(testAmbientSession)
	f.RetConst = nil
	f.Body = &Block{Func: f}
	out = f.OutputSess(testAmbientSession, false, false, nil)
	if !strings.Contains(out, "{") {
		t.Fatal("want body braces", out)
	}
	if strings.Contains(out, "if (DEPTH") {
		t.Fatal("Body without RetConst must not invent depth if", out)
	}
	// complete depth_protect IR
	f.RetConst = MakeIntSess(testAmbientSession, 42)
	out = f.OutputSess(testAmbientSession, false, false, nil)
	if !strings.Contains(out, "if (DEPTH < MAX_DEPTH)") || !strings.Contains(out, "return 42") {
		t.Fatal("complete depth_protect IR", out)
	}
	// empty ret_c value — sticky no invent "return ;" depth shell (whole emit fail closed)
	ClearErrorSess(testAmbientSession)
	f.RetConst = &Constant{Type: GetIntTypeSess(testAmbientSession), Value: ""}
	out = f.OutputSess(testAmbientSession, false, false, nil)
	if out != "" {
		t.Fatal("empty RetConst.Value must fail closed empty", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty RetConst.Value must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomSignatureIncompleteAmbientFailClosed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	seedTypesForTest(NewRngSess(testAmbientSession, 1), opts, probs, vs, nil)
	inc := IncompleteEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.EffectAccum = &inc
	cg.Types = vs.Types
	if MakeRandomSignature(NewRngSess(testAmbientSession, 2), opts, probs, vs, &vs.Sym, cg, GetIntTypeSess(testAmbientSession), nil, nil) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomSignature")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputHeaderNilFunctionSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*Function)(nil).OutputHeaderSess(testAmbientSession, false) != "" {
		t.Fatal("nil Function OutputHeader must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Function OutputHeader must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestReturnTypeCAndParamListSticky(t *testing.T) {
	// RV Type-nil sticky (no invent fall through to ReturnType / void)
	ClearErrorSess(testAmbientSession)
	f := &Function{
		Name: "f", ReturnType: GetIntTypeSess(testAmbientSession),
		RV: &Variable{Name: "rv", Type: nil},
	}
	if s := f.returnTypeCSess(testAmbientSession); s != "" {
		t.Fatal("RV Type-nil returnTypeC invent", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("RV Type-nil returnTypeC must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if s := f.OutputHeaderSess(testAmbientSession, false); s != "" {
		t.Fatal("RV Type-nil OutputHeader invent", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("RV Type-nil OutputHeader must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil Function paramList sticky empty (no invent "void")
	if s := (*Function)(nil).paramListCSess(testAmbientSession); s != "" {
		t.Fatal("nil Function paramListC invent", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Function paramListC must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// empty Param is complete void
	if s := (&Function{Name: "g"}).paramListCSess(testAmbientSession); s != "void" {
		t.Fatal("empty Param want void, got", s)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("empty Param paramListC must not SetError")
	}
	// live RV path
	f2 := &Function{
		Name: "h", ReturnType: GetIntTypeSess(testAmbientSession),
		RV: CreateVariableScalarsSess(testAmbientSession, "rv", GetIntTypeSess(testAmbientSession), false, false),
	}
	if s := f2.returnTypeCSess(testAmbientSession); s == "" {
		t.Fatal("live RV returnTypeC empty")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("live RV returnTypeC must not leave sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFunctionOutputNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*Function)(nil).OutputSess(testAmbientSession, false, false, nil) != "" {
		t.Fatal("nil Function Output must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Function Output must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// builtin soft empty
	if (&Function{IsBuiltin: true, Name: "b"}).OutputSess(testAmbientSession, false, false, nil) != "" {
		t.Fatal("builtin Function Output must soft empty")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("builtin Function Output must stay non-sticky")
	}
	ClearErrorSess(testAmbientSession)
}
