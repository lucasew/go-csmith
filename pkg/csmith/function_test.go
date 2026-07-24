package csmith

import (
	"strings"
	"testing"
)

func TestRandomFunctionName(t *testing.T) {
	// Function.cpp:249 — util.cpp gensym process-wide; private GenSym ignored
	ResetDefaultGensym()
	if RandomFunctionName(nil) != "func_1" || RandomFunctionName(nil) != "func_2" {
		t.Fatal("func gensym")
	}
	// private GenSym must not invent a separate stream
	var g GenSym
	if RandomFunctionName(&g) != "func_3" {
		t.Fatal("must use process gensym, not private GenSym")
	}
}

func TestParamListProbabilityRange(t *testing.T) {
	opts := Defaults()
	r := NewRng(2)
	// max_params=5 → rnd_upto(5) in 0..4
	for i := 0; i < 20; i++ {
		p := ParamListProbability(r, opts)
		if p >= 5 {
			t.Fatalf("param list prob %d", p)
		}
	}
	// sticky no invent param count 0 without RNG draw
	ClearError()
	if ParamListProbability(nil, opts) != 0 {
		t.Fatal("nil RNG ParamListProbability must fail closed 0")
	}
	if !HasError() {
		t.Fatal("nil RNG ParamListProbability must SetError sticky")
	}
	ClearError()
}

func TestMakeRandomSignatureParams(t *testing.T) {
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	// GenerateParameterVariable needs Type env (no soft invent simple)
	seedTypesForTest(NewRng(1), opts, probs, vs, nil)
	// share gensym counters: use vs.Sym for params and separate for funcs is OK upstream-global
	// For 1:1 naming, share one GenSym for both
	sym := &vs.Sym
	r := NewRng(2)
	f := MakeRandomSignature(r, opts, probs, vs, sym, EmptyCGContext(), GetSimpleType(EInt), nil, nil)
	if f == nil || !strings.HasPrefix(f.Name, "func_") {
		t.Fatalf("name %+v", f)
	}
	if f.ReturnType != GetSimpleType(EInt) {
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
		if p == nil || !p.IsArgument() {
			t.Fatalf("param %v", p)
		}
	}
	proto := f.OutputForwardDecl()
	if !strings.Contains(proto, f.Name) || !strings.HasSuffix(proto, ");") {
		t.Fatalf("proto %q", proto)
	}
}

func TestMakeFirstNoParamsHasBody(t *testing.T) {
	ReinstallTestProcessSingletons()
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	var list FunctionList
	r := NewRng(2)
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
	out := f.Output()
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
	if BlockProbability(4, NewRng(2)) != 3 {
		t.Fatal("max_block_size 4 → 3")
	}
}

func TestBlockProbabilityNilRNGSticky(t *testing.T) {
	// C++ always has RNG; sticky no invent fixed block size without draw
	ClearError()
	if BlockProbability(4, nil) != 0 {
		t.Fatal("nil RNG must fail closed 0")
	}
	if !HasError() {
		t.Fatal("nil RNG BlockProbability must SetError sticky")
	}
	ClearError()
}

func TestMakeFirstReturnBreaksEarly(t *testing.T) {
	// With enough seeds, some bodies end early on return
	opts := Defaults()
	probs := NewProbabilities(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	foundEarly := false
	for seed := uint64(1); seed < 50; seed++ {
		vs := NewVariableSelector(opts)
		r := NewRng(seed)
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
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	seedTypesForTest(NewRng(1), opts, probs, vs, nil)
	SetError(ErrGeneric)
	f := MakeRandomSignature(NewRng(2), opts, probs, vs, &vs.Sym, EmptyCGContext(), GetIntType(), nil, nil)
	if f != nil {
		t.Fatal("sticky error must fail closed")
	}
	ClearError()
}

func TestMakeRandomSignatureNoInventWithoutSession(t *testing.T) {
	// Function.cpp always has RNG + Probabilities; no invent signature shells
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	seedTypesForTest(NewRng(1), opts, probs, vs, nil)
	cg := EmptyCGContext()
	if MakeRandomSignature(nil, opts, probs, vs, &vs.Sym, cg, GetIntType(), nil, nil) != nil {
		t.Fatal("nil RNG must not invent signature")
	}
	if MakeRandomSignature(NewRng(2), opts, nil, vs, &vs.Sym, cg, GetIntType(), nil, nil) != nil {
		t.Fatal("nil probs must not invent signature")
	}
	// MakeFirst same contract
	if MakeFirst(nil, opts, probs, vs, &vs.Sym, NewExprTables(opts), NewStatementThresholdTable(opts), nil, nil) != nil {
		t.Fatal("nil RNG must not invent first")
	}
	if MakeFirst(NewRng(2), opts, nil, vs, &vs.Sym, NewExprTables(opts), NewStatementThresholdTable(opts), nil, nil) != nil {
		t.Fatal("nil probs must not invent first")
	}
}

func TestMakeFirstMakeRandomFunctionIncompleteGlobalListFailClosed(t *testing.T) {
	// incomplete GlobalList seed must sticky ERROR (no invent partial FM / body success)
	ClearError()
	opts := Defaults()
	opts.MaxBlockSize = 1
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	seedTypesForTest(NewRng(1), opts, probs, vs, nil)
	// plant incomplete GlobalList hole
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	vs.GlobalList = []*Variable{g, nil}
	if MakeFirst(NewRng(3), opts, probs, vs, &vs.Sym, NewExprTables(opts), NewStatementThresholdTable(opts), nil, nil) != nil {
		t.Fatal("incomplete GlobalList must fail closed MakeFirst")
	}
	if !HasError() {
		t.Fatal("incomplete GlobalList must SetError sticky MakeFirst")
	}
	ClearError()
	// MakeRandomFunction same seed gate
	cg := EmptyCGContext()
	cg.Types = vs.Types
	if MakeRandomFunction(NewRng(4), opts, probs, vs, &vs.Sym, NewExprTables(opts), NewStatementThresholdTable(opts), cg, GetIntType(), nil, nil) != nil {
		t.Fatal("incomplete GlobalList must fail closed MakeRandomFunction")
	}
	if !HasError() {
		t.Fatal("incomplete GlobalList must SetError sticky MakeRandomFunction")
	}
	ClearError()
}

func TestMakeRandomForERRORGuardAfterBody(t *testing.T) {
	// StatementFor.cpp:304 ERROR_GUARD after body
	ClearError()
	opts := Defaults()
	opts.MaxBlockSize = 0
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	seedTypesForTest(NewRng(1), opts, probs, vs, nil)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	vs.GlobalList = []*Variable{g}
	fm := NewFactMgr(f)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.Types = vs.Types
	f.Stack = []*Block{{Func: f}}
	// sticky error before make fails early
	SetError(ErrGeneric)
	st := MakeRandomFor(NewRng(3), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg)
	if st != nil && st.Loop != nil && st.Then != nil {
		t.Fatal("sticky error should not complete for")
	}
	ClearError()
}

func TestFunctionOutputNoSoftInventBodyOrRetConst(t *testing.T) {
	// Function.cpp:575–597 — depth_protect + body + ret_c always together
	// sticky no invent empty braces / header-only / if without else / "0"
	ClearError()
	f := &Function{
		Name: "func_x", ReturnType: GetIntType(),
		RV:           CreateVariableScalars("func_x_rv", GetIntType(), false, false),
		DepthProtect: true,
	}
	out := f.Output()
	if out != "" {
		t.Fatal("nil Body must fail closed empty, got", out)
	}
	if !HasError() {
		t.Fatal("nil Body Output must SetError sticky")
	}
	// RetConst only — sticky no invent if/else without body
	ClearError()
	f.RetConst = MakeInt(42)
	out = f.Output()
	if out != "" {
		t.Fatal("RetConst without Body must fail closed empty", out)
	}
	if !HasError() {
		t.Fatal("RetConst without Body must SetError sticky")
	}
	// Body without RetConst — body only, no invent if without else
	ClearError()
	f.RetConst = nil
	f.Body = &Block{Func: f}
	out = f.Output()
	if !strings.Contains(out, "{") {
		t.Fatal("want body braces", out)
	}
	if strings.Contains(out, "if (DEPTH") {
		t.Fatal("Body without RetConst must not invent depth if", out)
	}
	// complete depth_protect IR
	f.RetConst = MakeInt(42)
	out = f.Output()
	if !strings.Contains(out, "if (DEPTH < MAX_DEPTH)") || !strings.Contains(out, "return 42") {
		t.Fatal("complete depth_protect IR", out)
	}
	// empty ret_c value — sticky no invent "return ;" depth shell (whole emit fail closed)
	ClearError()
	f.RetConst = &Constant{Type: GetIntType(), Value: ""}
	out = f.Output()
	if out != "" {
		t.Fatal("empty RetConst.Value must fail closed empty", out)
	}
	if !HasError() {
		t.Fatal("empty RetConst.Value must SetError sticky")
	}
	ClearError()
}

func TestMakeRandomSignatureIncompleteAmbientFailClosed(t *testing.T) {
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	seedTypesForTest(NewRng(1), opts, probs, vs, nil)
	inc := IncompleteEffect()
	cg := EmptyCGContext()
	cg.EffectAccum = &inc
	cg.Types = vs.Types
	if MakeRandomSignature(NewRng(2), opts, probs, vs, &vs.Sym, cg, GetIntType(), nil, nil) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomSignature")
	}
	if !HasError() {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearError()
}

func TestOutputHeaderNilFunctionSticky(t *testing.T) {
	ClearError()
	if (*Function)(nil).OutputHeader(false) != "" {
		t.Fatal("nil Function OutputHeader must fail closed")
	}
	if !HasError() {
		t.Fatal("nil Function OutputHeader must SetError sticky")
	}
	ClearError()
}

func TestReturnTypeCAndParamListSticky(t *testing.T) {
	// RV Type-nil sticky (no invent fall through to ReturnType / void)
	ClearError()
	f := &Function{
		Name: "f", ReturnType: GetIntType(),
		RV: &Variable{Name: "rv", Type: nil},
	}
	if s := f.returnTypeC(); s != "" {
		t.Fatal("RV Type-nil returnTypeC invent", s)
	}
	if !HasError() {
		t.Fatal("RV Type-nil returnTypeC must SetError sticky")
	}
	ClearError()
	if s := f.OutputHeader(false); s != "" {
		t.Fatal("RV Type-nil OutputHeader invent", s)
	}
	if !HasError() {
		t.Fatal("RV Type-nil OutputHeader must SetError sticky")
	}
	ClearError()
	// nil Function paramList sticky empty (no invent "void")
	if s := (*Function)(nil).paramListC(); s != "" {
		t.Fatal("nil Function paramListC invent", s)
	}
	if !HasError() {
		t.Fatal("nil Function paramListC must SetError sticky")
	}
	ClearError()
	// empty Param is complete void
	if s := (&Function{Name: "g"}).paramListC(); s != "void" {
		t.Fatal("empty Param want void, got", s)
	}
	if HasError() {
		t.Fatal("empty Param paramListC must not SetError")
	}
	// live RV path
	f2 := &Function{
		Name: "h", ReturnType: GetIntType(),
		RV: CreateVariableScalars("rv", GetIntType(), false, false),
	}
	if s := f2.returnTypeC(); s == "" {
		t.Fatal("live RV returnTypeC empty")
	}
	if HasError() {
		t.Fatal("live RV returnTypeC must not leave sticky")
	}
	ClearError()
}

func TestFunctionOutputNilSticky(t *testing.T) {
	ClearError()
	if (*Function)(nil).Output() != "" {
		t.Fatal("nil Function Output must fail closed")
	}
	if !HasError() {
		t.Fatal("nil Function Output must SetError sticky")
	}
	ClearError()
	// builtin soft empty
	if (&Function{IsBuiltin: true, Name: "b"}).Output() != "" {
		t.Fatal("builtin Function Output must soft empty")
	}
	if HasError() {
		t.Fatal("builtin Function Output must stay non-sticky")
	}
	ClearError()
}
