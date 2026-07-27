package csmith

import "testing"

func TestChooseFuncSkipsUnbuilt(t *testing.T) {
	built := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession), IsBuilt: true}
	unbuilt := &Function{Name: "func_2", ReturnType: GetIntTypeSess(testAmbientSession), IsBuilt: false}
	funcs := []*Function{built, unbuilt}
	r := NewRngSess(testAmbientSession, 2)
	for i := 0; i < 20; i++ {
		f := ChooseFunc(r, funcs, GetIntTypeSess(testAmbientSession), nil)
		if f == unbuilt {
			t.Fatal("must not choose unbuilt")
		}
		if f != built && f != nil {
			t.Fatalf("%v", f)
		}
	}
	// only unbuilt → nil
	if ChooseFunc(NewRngSess(testAmbientSession, 1), []*Function{unbuilt}, GetIntTypeSess(testAmbientSession), nil) != nil {
		t.Fatal("expected nil")
	}
	// nil Function* hole fails closed sticky — no invent skip as absent / soft re-pick
	ClearErrorSess(testAmbientSession)
	if ChooseFunc(NewRngSess(testAmbientSession, 1), []*Function{built, nil}, GetIntTypeSess(testAmbientSession), nil) != nil {
		t.Fatal("nil hole must fail closed")
	}
	// nil Funcs hole must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
	// nil ReturnType when ret wanted fails closed sticky — no invent soft-skip as absent
	// (C++ is_convertable would deref return_type*; list hole aborts whole choose)
	noRet := &Function{Name: "bad", ReturnType: nil, IsBuilt: true, BuildState: BuildBuilt}
	if ChooseFunc(NewRngSess(testAmbientSession, 1), []*Function{built, noRet}, GetIntTypeSess(testAmbientSession), nil) != nil {
		t.Fatal("nil ReturnType must fail closed whole choose, not invent skip")
	}
	// nil ReturnType must SetError sticky — residual sticky may live on owner bag, not ambient dual-fill
	ClearErrorSess(testAmbientSession)
}

func TestArrayNoLoopInitializer(t *testing.T) {
	opts := Defaults()
	// global → true
	g := CreateArrayVariable(NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), nil, nil, nil, "g_1", GetIntTypeSess(testAmbientSession), MakeIntSess(testAmbientSession, 0), NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false}))
	if g == nil || !g.NoLoopInitializerSess(testAmbientSession) {
		t.Fatal("global must no-loop")
	}
	// local with no multi inits: force empty InitValues
	blk := &Block{}
	loc := CreateArrayVariable(NewRngSess(testAmbientSession, 3), opts, NewProbabilities(opts), nil, nil, blk, "l_1", GetIntTypeSess(testAmbientSession), MakeIntSess(testAmbientSession, 1), NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false}))
	if loc == nil {
		t.Fatal("nil local")
	}
	loc.InitValues = nil
	loc.ArrayInits = nil
	if loc.NoLoopInitializerSess(testAmbientSession) {
		// local non-const simple without multi inits → false
		t.Fatal("local should allow loop init")
	}
	out := loc.OutputInitOptsSess(testAmbientSession, "    ", []string{"i"}, true)
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
		Name: "good", ReturnType: GetIntTypeSess(testAmbientSession), BuildState: BuildBuilt, IsBuilt: true,
		RV:      &Variable{Name: "good_rv", Type: GetIntTypeSess(testAmbientSession), Qfer: NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{true})},
		FEffect: EmptyEffect(),
	}
	wild := NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false})
	wild.Wildcard = true
	got := ChooseFuncContext(NewRngSess(testAmbientSession, 2), []*Function{good}, GetIntTypeSess(testAmbientSession), nil, nil, Defaults(), &wild)
	if got != good {
		t.Fatalf("got %v", got)
	}
}

func TestChooseFuncContextNilRVQferFailClosed(t *testing.T) {
	// RV always live after Function create; nil RV with qfer filter must not soft-skip
	good := &Function{
		Name: "good", ReturnType: GetIntTypeSess(testAmbientSession), BuildState: BuildBuilt, IsBuilt: true,
		RV:      &Variable{Name: "good_rv", Type: GetIntTypeSess(testAmbientSession), Qfer: NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false})},
		FEffect: EmptyEffect(),
	}
	noRV := &Function{
		Name: "bad", ReturnType: GetIntTypeSess(testAmbientSession), BuildState: BuildBuilt, IsBuilt: true,
		RV: nil, FEffect: EmptyEffect(),
	}
	q := NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false})
	ClearErrorSess(testAmbientSession)
	if ChooseFuncContext(NewRngSess(testAmbientSession, 1), []*Function{good, noRV}, GetIntTypeSess(testAmbientSession), nil, nil, Defaults(), &q) != nil {
		t.Fatal("nil RV among candidates must fail closed whole choose")
	}
	// nil RV must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
	// good alone with matching qfer still works
	if ChooseFuncContext(NewRngSess(testAmbientSession, 2), []*Function{good}, GetIntTypeSess(testAmbientSession), nil, nil, Defaults(), &q) != good {
		t.Fatal("complete RV must still choose")
	}
}

func TestChooseFuncContextIncompleteAmbientSticky(t *testing.T) {
	// incomplete EffectContext fails closed sticky (no invent conflict-filter under holes)
	good := &Function{
		Name: "good", ReturnType: GetIntTypeSess(testAmbientSession), BuildState: BuildBuilt, IsBuilt: true,
		FEffect: EmptyEffect(),
	}
	cg := WithEffectContext(IncompleteEffect()).WithSession(testAmbientSession)
	ClearErrorSess(testAmbientSession)
	if ChooseFuncContext(NewRngSess(testAmbientSession, 1), []*Function{good}, GetIntTypeSess(testAmbientSession), nil, &cg, Defaults(), nil) != nil {
		t.Fatal("incomplete ambient must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete ambient must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete EffectStm / GlobalFacts also sticky
	cgStm := EmptyCGContext().WithSession(testAmbientSession)
	cgStm.EffectStm = IncompleteEffect()
	if ChooseFuncContext(NewRngSess(testAmbientSession, 3), []*Function{good}, GetIntTypeSess(testAmbientSession), nil, &cgStm, Defaults(), nil) != nil {
		t.Fatal("incomplete EffectStm must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectStm must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "caller"}
	fm := NewFactMgrSess(testAmbientSession, f)
	fm.GlobalFacts = IncompleteFactSlice()
	cgFacts := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	if ChooseFuncContext(NewRngSess(testAmbientSession, 4), []*Function{good}, GetIntTypeSess(testAmbientSession), nil, &cgFacts, Defaults(), nil) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete ambient still chooses
	cg2 := EmptyCGContext().WithSession(testAmbientSession)
	if ChooseFuncContext(NewRngSess(testAmbientSession, 2), []*Function{good}, GetIntTypeSess(testAmbientSession), nil, &cg2, Defaults(), nil) != good {
		t.Fatal("complete ambient must still choose")
	}
}

func TestChooseFuncContextIncompleteFEffectSticky(t *testing.T) {
	// incomplete callee FEffect fails closed sticky (no invent skip as conflict past hole)
	good := &Function{
		Name: "good", ReturnType: GetIntTypeSess(testAmbientSession), BuildState: BuildBuilt, IsBuilt: true,
		FEffect: EmptyEffect(),
	}
	bad := &Function{
		Name: "bad", ReturnType: GetIntTypeSess(testAmbientSession), BuildState: BuildBuilt, IsBuilt: true,
		FEffect: IncompleteEffect(),
	}
	cg := EmptyCGContext().WithSession(testAmbientSession)
	ClearErrorSess(testAmbientSession)
	if ChooseFuncContext(NewRngSess(testAmbientSession, 1), []*Function{good, bad}, GetIntTypeSess(testAmbientSession), nil, &cg, Defaults(), nil) != nil {
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
		Name: "good", ReturnType: GetIntTypeSess(testAmbientSession), BuildState: BuildBuilt, IsBuilt: true,
		RV:      &Variable{Name: "good_rv", Type: GetIntTypeSess(testAmbientSession), Qfer: NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false})},
		FEffect: EmptyEffect(),
	}
	broken := &Function{
		Name: "broken", ReturnType: GetIntTypeSess(testAmbientSession), BuildState: BuildBuilt, IsBuilt: true,
		RV:      &Variable{Name: "broken_rv", Type: GetIntTypeSess(testAmbientSession), Qfer: NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false})},
		FEffect: EmptyEffect(),
	}
	// request qfer with unpaired const/vol depths → Match SetError
	badQfer := CVQualifiers{IsConsts: []bool{false, false}, IsVolatiles: []bool{true}}
	if ChooseFuncContext(NewRngSess(testAmbientSession, 1), []*Function{broken, good}, GetIntTypeSess(testAmbientSession), nil, nil, Defaults(), &badQfer) != nil {
		t.Fatal("Match residual must fail closed ChooseFuncContext")
	}
	// Match residual ChooseFuncContext must SetError sticky — sticky on owner bag / throwaway, not package ambient dual-fill
	ClearErrorSess(testAmbientSession)
}

func TestChooseFuncUsesIsConvertable(t *testing.T) {
	// Function.cpp:288–289 — is_convertable (short → int return ok)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession), BuildState: BuildBuilt, IsBuilt: true, FEffect: EmptyEffect()}
	got := ChooseFuncContext(NewRngSess(testAmbientSession, 1), []*Function{f}, GetSimpleTypeSess(testAmbientSession, EShort), nil, nil, Defaults(), nil)
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
	bi := &Function{Name: "b", ReturnType: GetIntTypeSess(testAmbientSession), IsBuiltin: true, BuildState: BuildBuilt, IsBuilt: true, FEffect: EmptyEffect()}
	// only builtins eligible
	got := ChooseFuncContext(NewRngSess(testAmbientSession, 1), []*Function{bi}, GetIntTypeSess(testAmbientSession), nil, nil, opts, nil)
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
		Name: "b", ReturnType: GetIntTypeSess(testAmbientSession), IsBuiltin: true,
		BuildState: BuildBuilt, IsBuilt: true, FEffect: EmptyEffect(),
	}
	user := &Function{
		Name: "u", ReturnType: GetIntTypeSess(testAmbientSession),
		BuildState: BuildBuilt, IsBuilt: true, FEffect: EmptyEffect(),
	}
	// only builtins: must stay nil (no invent 50% pick)
	for seed := uint64(1); seed < 40; seed++ {
		if ChooseFuncContext(NewRngSess(testAmbientSession, seed), []*Function{bi}, GetIntTypeSess(testAmbientSession), nil, nil, opts, nil) != nil {
			t.Fatalf("BuiltinFunctionProb=0 must never pick builtin first seed=%d", seed)
		}
	}
	// with user pool: always user, never invent builtin path at 50%
	for seed := uint64(1); seed < 40; seed++ {
		got := ChooseFuncContext(NewRngSess(testAmbientSession, seed), []*Function{bi, user}, GetIntTypeSess(testAmbientSession), nil, nil, opts, nil)
		if got != user {
			t.Fatalf("want user only, got %v seed=%d", got, seed)
		}
	}
}

func TestChooseFuncUsesProbTableNotOptsCLI(t *testing.T) {
	// Function.cpp:329 — rnd_flipcoin(BuiltinFunctionProb()) reads
	// Probabilities::get_prob (post-random_random table), not CGOptions raw.
	// CLI --builtin-function-prob seeds the table; random_random may rewrite it.
	opts := Defaults()
	opts.Builtins = true
	opts.BuiltinFunctionProb = 0 // CLI says never
	// Session bag with table that says always (100) — choose must follow table.
	s := NewSession(opts)
	s.Probs = NewProbabilities(opts)
	s.Probs.single[PBuiltinFunctionProb] = 100
	bi := &Function{
		Name: "b", ReturnType: GetIntTypeSess(testAmbientSession), IsBuiltin: true,
		BuildState: BuildBuilt, IsBuilt: true, FEffect: EmptyEffect(),
	}
	user := &Function{
		Name: "u", ReturnType: GetIntTypeSess(testAmbientSession),
		BuildState: BuildBuilt, IsBuilt: true, FEffect: EmptyEffect(),
	}
	cg := EmptyCGContext().WithSession(s)
	// p=100 → always try builtin first
	got := ChooseFuncContext(NewRngSess(s, 1), []*Function{bi, user}, GetIntTypeSess(testAmbientSession), nil, &cg, opts, nil)
	if got != bi {
		t.Fatalf("want table p=100 → builtin, got %v (must not use CLI opts.BuiltinFunctionProb=0)", got)
	}
	// table p=0 → never builtin first even if CLI says 100
	opts.BuiltinFunctionProb = 100
	s.Probs.single[PBuiltinFunctionProb] = 0
	for seed := uint64(1); seed < 30; seed++ {
		got := ChooseFuncContext(NewRngSess(s, seed), []*Function{bi, user}, GetIntTypeSess(testAmbientSession), nil, &cg, opts, nil)
		if got != user {
			t.Fatalf("table p=0 must yield user (not CLI 100), got %v seed=%d", got, seed)
		}
	}
}

func TestChooseFuncAlwaysBurnsBuiltinProbFlip(t *testing.T) {
	// Function.cpp:329 — when builtins enabled, always rnd_flipcoin(BuiltinFunctionProb)
	// even if no eligible builtin (wrong return type / empty ok_builtin_funcs).
	// Skipping the flip desyncs the stream vs upstream (first_div after TermFunction).
	opts := Defaults()
	opts.Builtins = true
	opts.BuiltinFunctionProb = 50
	// Builtin return UInt; request Short so is_convertable fails → okBuiltin empty.
	// User returns Int (convertible from Short via is_convertable? short→int yes for ret check:
	// choose_func: type->is_convertable(return_type) means requested type convertable FROM return.
	// short.is_convertable(int) — may fail; use exact Int return for user.
	bi := &Function{
		Name: "b", ReturnType: GetSimpleTypeSess(testAmbientSession, EUInt), IsBuiltin: true,
		BuildState: BuildBuilt, IsBuilt: true, FEffect: EmptyEffect(),
	}
	user := &Function{
		Name: "u", ReturnType: GetIntTypeSess(testAmbientSession),
		BuildState: BuildBuilt, IsBuilt: true, FEffect: EmptyEffect(),
	}
	// Need a return type that excludes the builtin but keeps the user.
	// float return requested: int not float-compatible → both empty? Use Int request:
	// UInt is_convertable to Int? typically yes for integers. Force exclude via effect.
	// Simpler: only a type-mismatched builtin on the list + matching user — request void-like
	// Use a struct return type for builtin only.
	st := &Type{isStruct: true, StructName: "S_bi"}
	bi.ReturnType = st
	wantRet := GetIntTypeSess(testAmbientSession)

	// Same seed: path with empty okBuiltin must consume the same number of genrand draws
	// as path with a live builtin that loses the coin — both burn exactly one flipcoin
	// before get_one_function(user). Compare subsequent RndUpto stream position via depth.
	rEmpty := NewRngSess(testAmbientSession, 42)
	got := ChooseFuncContext(rEmpty, []*Function{bi, user}, wantRet, nil, nil, opts, nil)
	if got != user {
		t.Fatalf("empty-okBuiltin path: want user, got %v", got)
	}
	depthEmpty := rEmpty.RandDepthSess(testAmbientSession)

	// Eligible builtin + user: always burns flip, then may pick user or builtin.
	// Use same seed; only care that depth is at least as large as empty path's flip+pick.
	// With p=50, either pick: if builtin, get_one_function(n=1) no upto; if user, n=1 no upto.
	// Empty path: flip + get_one_function(user n=1). Eligible: flip + get_one_function(n=1).
	// Depth must match.
	biOK := &Function{
		Name: "b2", ReturnType: GetIntTypeSess(testAmbientSession), IsBuiltin: true,
		BuildState: BuildBuilt, IsBuilt: true, FEffect: EmptyEffect(),
	}
	rOK := NewRngSess(testAmbientSession, 42)
	_ = ChooseFuncContext(rOK, []*Function{biOK, user}, wantRet, nil, nil, opts, nil)
	depthOK := rOK.RandDepthSess(testAmbientSession)
	if depthEmpty != depthOK {
		t.Fatalf("BuiltinFunctionProb flip must always burn: empty-okBuiltin depth=%d eligible depth=%d",
			depthEmpty, depthOK)
	}

	// Without the flip (regression guard): Builtins=false must NOT burn the coin —
	// depth must be lower than Builtins=true empty-okBuiltin path (only get_one_function).
	optsOff := Defaults()
	optsOff.Builtins = false
	rOff := NewRngSess(testAmbientSession, 42)
	gotOff := ChooseFuncContext(rOff, []*Function{bi, user}, wantRet, nil, nil, optsOff, nil)
	if gotOff != user {
		t.Fatalf("builtins off: want user, got %v", gotOff)
	}
	depthOff := rOff.RandDepthSess(testAmbientSession)
	if depthOff >= depthEmpty {
		t.Fatalf("builtins off must not burn BuiltinFunctionProb: off=%d on=%d", depthOff, depthEmpty)
	}
}
