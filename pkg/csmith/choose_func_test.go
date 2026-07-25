package csmith

import "testing"

func TestChooseFuncSkipsUnbuilt(t *testing.T) {
	built := &Function{Name: "func_1", ReturnType: GetIntType(), IsBuilt: true}
	unbuilt := &Function{Name: "func_2", ReturnType: GetIntType(), IsBuilt: false}
	funcs := []*Function{built, unbuilt}
	r := NewRng(2)
	for i := 0; i < 20; i++ {
		f := ChooseFunc(r, funcs, GetIntType(), nil)
		if f == unbuilt {
			t.Fatal("must not choose unbuilt")
		}
		if f != built && f != nil {
			t.Fatalf("%v", f)
		}
	}
	// only unbuilt → nil
	if ChooseFunc(NewRng(1), []*Function{unbuilt}, GetIntType(), nil) != nil {
		t.Fatal("expected nil")
	}
	// nil Function* hole fails closed sticky — no invent skip as absent / soft re-pick
	ClearErrorSess(testAmbientSession)
	if ChooseFunc(NewRng(1), []*Function{built, nil}, GetIntType(), nil) != nil {
		t.Fatal("nil hole must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Funcs hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil ReturnType when ret wanted fails closed sticky — no invent soft-skip as absent
	// (C++ is_convertable would deref return_type*; list hole aborts whole choose)
	noRet := &Function{Name: "bad", ReturnType: nil, IsBuilt: true, BuildState: BuildBuilt}
	if ChooseFunc(NewRng(1), []*Function{built, noRet}, GetIntType(), nil) != nil {
		t.Fatal("nil ReturnType must fail closed whole choose, not invent skip")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil ReturnType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestArrayNoLoopInitializer(t *testing.T) {
	opts := Defaults()
	// global → true
	g := CreateArrayVariable(NewRng(2), opts, NewProbabilities(opts), nil, nil, nil, "g_1", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
	if g == nil || !g.NoLoopInitializer() {
		t.Fatal("global must no-loop")
	}
	// local with no multi inits: force empty InitValues
	blk := &Block{}
	loc := CreateArrayVariable(NewRng(3), opts, NewProbabilities(opts), nil, nil, blk, "l_1", GetIntType(), MakeInt(1), NewCVQualifiers([]bool{false}, []bool{false}))
	if loc == nil {
		t.Fatal("nil local")
	}
	loc.InitValues = nil
	loc.ArrayInits = nil
	if loc.NoLoopInitializer() {
		// local non-const simple without multi inits → false
		t.Fatal("local should allow loop init")
	}
	out := loc.OutputInit("    ", []string{"i"})
	if out == "" || !contains(out, "for") || !contains(out, "for (i = 0") {
		t.Fatal(out)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		})())
}

func TestChooseFuncContextQferWildcard(t *testing.T) {
	// Function.cpp:294–295 — qfer when set; Wildcard accepts any RV qfer
	good := &Function{
		Name: "good", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true,
		RV:      &Variable{Name: "good_rv", Type: GetIntType(), Qfer: NewCVQualifiers([]bool{false}, []bool{true})},
		FEffect: EmptyEffect(),
	}
	wild := NewCVQualifiers([]bool{false}, []bool{false})
	wild.Wildcard = true
	got := ChooseFuncContext(NewRng(2), []*Function{good}, GetIntType(), nil, nil, Defaults(), &wild)
	if got != good {
		t.Fatalf("got %v", got)
	}
}

func TestChooseFuncContextNilRVQferFailClosed(t *testing.T) {
	// RV always live after Function create; nil RV with qfer filter must not soft-skip
	good := &Function{
		Name: "good", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true,
		RV:      &Variable{Name: "good_rv", Type: GetIntType(), Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		FEffect: EmptyEffect(),
	}
	noRV := &Function{
		Name: "bad", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true,
		RV: nil, FEffect: EmptyEffect(),
	}
	q := NewCVQualifiers([]bool{false}, []bool{false})
	ClearErrorSess(testAmbientSession)
	if ChooseFuncContext(NewRng(1), []*Function{good, noRV}, GetIntType(), nil, nil, Defaults(), &q) != nil {
		t.Fatal("nil RV among candidates must fail closed whole choose")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RV must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// good alone with matching qfer still works
	if ChooseFuncContext(NewRng(2), []*Function{good}, GetIntType(), nil, nil, Defaults(), &q) != good {
		t.Fatal("complete RV must still choose")
	}
}

func TestChooseFuncContextIncompleteAmbientSticky(t *testing.T) {
	// incomplete EffectContext fails closed sticky (no invent conflict-filter under holes)
	good := &Function{
		Name: "good", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true,
		FEffect: EmptyEffect(),
	}
	cg := WithEffectContext(IncompleteEffect())
	ClearErrorSess(testAmbientSession)
	if ChooseFuncContext(NewRng(1), []*Function{good}, GetIntType(), nil, &cg, Defaults(), nil) != nil {
		t.Fatal("incomplete ambient must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete ambient must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete EffectStm / GlobalFacts also sticky
	cgStm := EmptyCGContext()
	cgStm.EffectStm = IncompleteEffect()
	if ChooseFuncContext(NewRng(3), []*Function{good}, GetIntType(), nil, &cgStm, Defaults(), nil) != nil {
		t.Fatal("incomplete EffectStm must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectStm must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "caller"}
	fm := NewFactMgrSess(testAmbientSession, f)
	fm.GlobalFacts = IncompleteFactSlice()
	cgFacts := EmptyCGContext().WithFactMgr(fm)
	if ChooseFuncContext(NewRng(4), []*Function{good}, GetIntType(), nil, &cgFacts, Defaults(), nil) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete ambient still chooses
	cg2 := EmptyCGContext()
	if ChooseFuncContext(NewRng(2), []*Function{good}, GetIntType(), nil, &cg2, Defaults(), nil) != good {
		t.Fatal("complete ambient must still choose")
	}
}

func TestChooseFuncContextIncompleteFEffectSticky(t *testing.T) {
	// incomplete callee FEffect fails closed sticky (no invent skip as conflict past hole)
	good := &Function{
		Name: "good", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true,
		FEffect: EmptyEffect(),
	}
	bad := &Function{
		Name: "bad", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true,
		FEffect: IncompleteEffect(),
	}
	cg := EmptyCGContext()
	ClearErrorSess(testAmbientSession)
	if ChooseFuncContext(NewRng(1), []*Function{good, bad}, GetIntType(), nil, &cg, Defaults(), nil) != nil {
		t.Fatal("incomplete FEffect among candidates must fail closed whole choose")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete FEffect must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestChooseFuncContextMatchResidualSticky(t *testing.T) {
	// unpaired qfer Match stickies residual ERROR; soft invent was continue then pick later good.
	// Fair: sticky fail closed whole choose.
	ClearErrorSess(testAmbientSession)
	good := &Function{
		Name: "good", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true,
		RV:      &Variable{Name: "good_rv", Type: GetIntType(), Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		FEffect: EmptyEffect(),
	}
	broken := &Function{
		Name: "broken", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true,
		RV:      &Variable{Name: "broken_rv", Type: GetIntType(), Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		FEffect: EmptyEffect(),
	}
	// request qfer with unpaired const/vol depths → Match SetError
	badQfer := CVQualifiers{IsConsts: []bool{false, false}, IsVolatiles: []bool{true}}
	if ChooseFuncContext(NewRng(1), []*Function{broken, good}, GetIntType(), nil, nil, Defaults(), &badQfer) != nil {
		t.Fatal("Match residual must fail closed ChooseFuncContext")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Match residual ChooseFuncContext must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestChooseFuncUsesIsConvertable(t *testing.T) {
	// Function.cpp:288–289 — is_convertable (short → int return ok)
	f := &Function{Name: "f", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true, FEffect: EmptyEffect()}
	got := ChooseFuncContext(NewRng(1), []*Function{f}, GetSimpleType(EShort), nil, nil, Defaults(), nil)
	if got != f {
		t.Fatal("short should convert via is_convertable to int return")
	}
}

func TestChooseFuncNoSoftBuiltinFallback(t *testing.T) {
	// Function.cpp:330–337 — after failed builtin coin / empty user pool, return null
	// (do not soft-fallback to builtins)
	opts := Defaults()
	opts.Builtins = true
	opts.BuiltinFunctionProb = 0 // never pick builtin first
	bi := &Function{Name: "b", ReturnType: GetIntType(), IsBuiltin: true, BuildState: BuildBuilt, IsBuilt: true, FEffect: EmptyEffect()}
	// only builtins eligible
	got := ChooseFuncContext(NewRng(1), []*Function{bi}, GetIntType(), nil, nil, opts, nil)
	if got != nil {
		t.Fatal("must not fall back to builtins when coin says user pool")
	}
}

func TestChooseFuncBuiltinProbZeroNoInvent50(t *testing.T) {
	// Function.cpp:329 uses BuiltinFunctionProb() as-is; 0 must not invent 50
	opts := Defaults()
	opts.Builtins = true
	opts.BuiltinFunctionProb = 0
	bi := &Function{
		Name: "b", ReturnType: GetIntType(), IsBuiltin: true,
		BuildState: BuildBuilt, IsBuilt: true, FEffect: EmptyEffect(),
	}
	user := &Function{
		Name: "u", ReturnType: GetIntType(),
		BuildState: BuildBuilt, IsBuilt: true, FEffect: EmptyEffect(),
	}
	// only builtins: must stay nil (no invent 50% pick)
	for seed := uint64(1); seed < 40; seed++ {
		if ChooseFuncContext(NewRng(seed), []*Function{bi}, GetIntType(), nil, nil, opts, nil) != nil {
			t.Fatalf("BuiltinFunctionProb=0 must never pick builtin first seed=%d", seed)
		}
	}
	// with user pool: always user, never invent builtin path at 50%
	for seed := uint64(1); seed < 40; seed++ {
		got := ChooseFuncContext(NewRng(seed), []*Function{bi, user}, GetIntType(), nil, nil, opts, nil)
		if got != user {
			t.Fatalf("want user only, got %v seed=%d", got, seed)
		}
	}
}
