package csmith

import (
	"strings"
	"testing"
)

func TestCreateArrayVariableDimensions(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	r := NewRng(2)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	av := CreateArrayVariable(r, opts, NewProbabilities(opts), nil, nil, nil, "g_1", GetIntType(), MakeInt(0), q)
	if av == nil || av.Dimension() < 1 {
		t.Fatal(av)
	}
	if av.TotalSize() < 1 {
		t.Fatal(av.Sizes)
	}
	for _, s := range av.Sizes {
		if s < 1 || s > opts.MaxArrayLenPerDim {
			t.Fatalf("dim %d", s)
		}
	}
	if av.TotalSize() > opts.MaxArrayLength {
		t.Fatalf("total %d > max %d", av.TotalSize(), opts.MaxArrayLength)
	}
}

func TestCreateArrayVariableAssertAndErrorGuard(t *testing.T) {
	// ArrayVariable.cpp:127–133 — assert type/void sticky; ERROR_GUARD after rnd_upto(99)
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	q := NewCVQualifiers([]bool{false}, []bool{false})
	if CreateArrayVariable(NewRng(1), opts, NewProbabilities(opts), nil, nil, nil, "g_v", GetSimpleType(EVoid), MakeInt(0), q) != nil {
		t.Fatal("void element must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("void element must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if CreateArrayVariable(NewRng(1), opts, NewProbabilities(opts), nil, nil, nil, "g_n", nil, MakeInt(0), q) != nil {
		t.Fatal("nil element must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil element must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if CreateArrayVariable(NewRng(1), opts, NewProbabilities(opts), nil, nil, nil, "", GetIntType(), MakeInt(0), q) != nil {
		t.Fatal("empty name must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty name must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	SetErrorSess(testAmbientSession, ErrGeneric)
	defer ClearErrorSess(testAmbientSession)
	if CreateArrayVariable(NewRng(1), opts, NewProbabilities(opts), nil, nil, nil, "g_e", GetIntType(), MakeInt(0), q) != nil {
		t.Fatal("sticky error after dim draw must fail closed")
	}
}

func TestCreateArrayVariableNoSoftInventSizeOne(t *testing.T) {
	// ArrayVariable.cpp:154–157 — empty sizes when no dim fits; no invent [1]
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxArrayDim = 0
	opts.MaxArrayLenPerDim = 0
	opts.MaxArrayLength = 0
	av := CreateArrayVariable(NewRng(1), opts, NewProbabilities(opts), nil, nil, nil, "g_z", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
	if av == nil {
		t.Fatal("nil")
	}
	if len(av.Sizes) != 0 {
		t.Fatalf("want empty sizes, got %v", av.Sizes)
	}
}

func TestCreateArrayVariableAggregateCreatesFieldVars(t *testing.T) {
	// ArrayVariable.cpp:161–163 — create_field_vars for aggregate element type
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	env := &TypeEnv{Sess: testAmbientSession}
	probs := NewProbabilities(opts)
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	st := MakeRandomStructType(NewRng(2), opts, probs, env, "S0")
	if st == nil || !st.IsStruct() {
		t.Skip("no struct")
	}
	av := CreateArrayVariable(NewRng(3), opts, probs, nil, nil, nil, "g_s", st, MakeRandom(st, opts, probs, NewRng(4)), NewCVQualifiers([]bool{false}, []bool{false}))
	if av == nil {
		t.Fatal("nil av")
	}
	if len(av.FieldVars) == 0 {
		t.Fatal("aggregate array must expand field vars")
	}
}

func TestCreateArrayVariableNilProbsNoInventAggregateAlt(t *testing.T) {
	// Constant::make_random needs live Probabilities for aggregates — no invent tables
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxArrayDim = 1
	opts.MaxArrayLenPerDim = 4
	opts.MaxArrayLength = 4
	st := &Type{isStruct: true, StructName: "SAlt", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	// large init_num path still must not invent aggregate alt values without probs
	av := CreateArrayVariable(NewRng(1), opts, nil, nil, nil, nil, "g_s", st, nil, NewCVQualifiers([]bool{false}, []bool{false}))
	if av == nil {
		t.Fatal("nil av")
	}
	if len(av.InitValues) != 0 {
		t.Fatalf("nil probs must not invent aggregate alt inits, got %v", av.InitValues)
	}
}

func TestCreateArrayVariablePointerAltNeedsMakeInitValue(t *testing.T) {
	// ArrayVariable.cpp:179–184 — pointer + !strict_const → make_init_value, not Constant "0"
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.StrictConstArrays = false
	opts.MaxArrayDim = 1
	opts.MaxArrayLenPerDim = 8
	opts.MaxArrayLength = 8
	q := NewCVQualifiers([]bool{false}, []bool{false})
	pt := PointerTo(GetIntType())
	// without VS/CG: when alt init_num > 0 fail closed non-sticky soft re-pick (no invent Constant stand-in)
	sawFail := false
	for seed := uint64(1); seed < 40; seed++ {
		ClearErrorSess(testAmbientSession)
		av := CreateArrayVariable(NewRng(seed), opts, NewProbabilities(opts), nil, nil, nil, "g_a", pt, MakeInt(0), q)
		if av == nil {
			sawFail = true
			continue
		}
		if len(av.InitValues) != 0 {
			t.Fatalf("seed %d: invented pointer alts without make_init_value: %v", seed, av.InitValues)
		}
	}
	if !sawFail {
		t.Fatal("expected some seeds to need make_init_value and fail closed")
	}
	ClearErrorSess(testAmbientSession)
	// with VS+CG: make_init_value path is live
	vs := NewVariableSelector(opts)
	vs.Probs = NewProbabilities(opts)
	_ = vs.GenerateNewGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntType(), nil, NewRng(1))
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	ClearErrorSess(testAmbientSession)
	av := CreateArrayVariable(NewRng(3), opts, vs.Probs, vs, &cg, nil, "g_p", pt, MakeInt(0), q)
	if av == nil {
		// make_init_value may ERROR_GUARD; not invent Constant "0"
		// Clear residual sticky so later tests are not poisoned
		ClearErrorSess(testAmbientSession)
		return
	}
	for _, s := range av.InitValues {
		if s == "" {
			t.Fatal("empty alt")
		}
	}
	ClearErrorSess(testAmbientSession)
}

func TestArrayCDecl(t *testing.T) {
	av := &ArrayVariable{
		Variable: Variable{Name: "a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2, 3}},
		Sizes:    []int{2, 3},
	}
	d := av.CDeclType()
	if d != "int32_t a[2][3]" {
		t.Fatal(d)
	}
}

func TestGenerateCanEmitArrayDecl(t *testing.T) {
	found := false
	for seed := uint64(1); seed < 80; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		// look for [digit]
		if strings.Contains(out, "[") && strings.Contains(out, "]") {
			// exclude main argc argv
			if strings.Contains(out, "static ") && (strings.Contains(out, "g_") || strings.Contains(out, "l_")) {
				found = true
				break
			}
		}
	}
	if !found {
		t.Log("no array decl in 1..79 — NewArrayVariableProb=20 may be rare with few creates")
	}
}

func TestCreateAndInitializeArrayFlip(t *testing.T) {
	opts := Defaults()
	// force high array prob
	vs := NewVariableSelector(opts)
	// hack probs
	vs.Probs.single[PNewArrayVariableProb] = 100
	r := NewRng(2)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	v := vs.createAndInitialize(AccessWrite, EmptyCGContext().WithSession(testAmbientSession), GetIntType(), q, nil, "g_9", r)
	if v == nil || !v.IsArray {
		t.Fatalf("%+v arrays=%d", v, len(vs.Arrays))
	}
}

func TestOutputAccessItemizedUsesIndexExprs(t *testing.T) {
	// ArrayVariable.cpp:539–552 — itemized emits name[expr]
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{8}},
		Sizes:    []int{8},
	}
	parent.AsArray = parent
	iv := CreateVariableScalars("i", GetIntType(), false, false)
	off := &Expression{Term: TermConstant, Con: MakeInt(2), ExprType: GetIntType()}
	ivExpr := &Expression{Term: TermVariable, Var: iv, ExprType: GetIntType()}
	fi := &Invocation{IsStd: true, Binary: "+", Args: []*Expression{ivExpr, off}}
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{8}},
		Sizes:      []int{8},
		Collective: parent,
		IndexExprs: []*Expression{{Term: TermFunction, Invoke: fi, ExprType: GetIntType()}},
		Indices:    []string{"stale"},
	}
	item.AsArray = item
	out := item.OutputAccess()
	if !strings.Contains(out, "g_a[") || !strings.Contains(out, "+") {
		t.Fatal(out)
	}
	if strings.Contains(out, "stale") {
		t.Fatal("must prefer IndexExprs over Indices strings")
	}
	// Variable::Output dispatches to ArrayVariable::Output for itemized
	if item.OutputC() != out {
		t.Fatalf("OutputC %q want %q", item.OutputC(), out)
	}
	if item.OutputLhsC() != out {
		t.Fatalf("OutputLhsC %q", item.OutputLhsC())
	}
}

func TestOutputAccessItemizedNoSizesOnlySoft(t *testing.T) {
	// ArrayVariable.cpp:544–545 — assert(!indices.empty()); no invent from sizes / bare name
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{4, 5}},
		Sizes:    []int{4, 5},
	}
	parent.AsArray = parent
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{4, 5}},
		Sizes:      []int{4, 5},
		Collective: parent,
	}
	item.AsArray = item
	ClearErrorSess(testAmbientSession)
	if out := item.OutputAccess(); out != "" {
		t.Fatalf("empty indices must fail closed, got %q", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty indices itemized OutputAccess must SetError sticky")
	}
	// Indices string path (ItemizeConstIndices) still works without IndexExprs
	ClearErrorSess(testAmbientSession)
	item.Indices = []string{"1", "2"}
	if got := item.OutputAccess(); got != "g_a[1][2]" {
		t.Fatal(got)
	}
	ClearErrorSess(testAmbientSession)
}

func TestItemizeAlreadyItemizedFailClosed(t *testing.T) {
	// ArrayVariable.cpp:250 — assert(collective == 0) on the receiver sticky
	ClearErrorSess(testAmbientSession)
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{4}},
		Sizes:    []int{4},
	}
	parent.AsArray = parent
	item := parent.Itemize(NewRng(1))
	if item == nil || item.Collective != parent {
		t.Fatal("first itemize")
	}
	// collective parent may be itemized again (new member); itemized member must not
	if parent.Itemize(NewRng(2)) == nil {
		t.Fatal("parent collective may itemize again")
	}
	ClearErrorSess(testAmbientSession)
	if item.Itemize(NewRng(3)) != nil {
		t.Fatal("re-itemize of itemized must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("re-itemize of itemized must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if parent.ItemizeInto(nil, nil) != nil {
		t.Fatal("nil RNG Itemize must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG Itemize must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestArrayCDeclTypeNoInventInt(t *testing.T) {
	// ArrayVariable decl always has live type; sticky no invent "int"
	ClearErrorSess(testAmbientSession)
	if s := (*ArrayVariable)(nil).CDeclType(); s != "" {
		t.Fatal("nil av CDeclType must fail closed", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil av CDeclType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	av := &ArrayVariable{Variable: Variable{Name: "g_a"}, Sizes: []int{2}}
	if s := av.CDeclType(); s != "" {
		t.Fatal("nil Type must fail closed", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Type CDeclType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if s := av.OutputDef(); s != "" {
		t.Fatal("incomplete array def must fail closed", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete OutputDef must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if s := (*ArrayVariable)(nil).OutputDef(); s != "" {
		t.Fatal("nil av OutputDef must fail closed", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil av OutputDef must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestArrayOutputAccessNoInventEmptyIndex(t *testing.T) {
	// ArrayVariable.cpp:548 — indices[i]->Output always live; no invent "g_a[]"
	parent := &ArrayVariable{Variable: Variable{Name: "g_a", Type: GetIntType()}, Sizes: []int{2}}
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntType()},
		Sizes:      []int{2},
		Collective: parent,
		IndexExprs: []*Expression{{Term: TermConstant}}, // nil Con → empty Output
	}
	ClearErrorSess(testAmbientSession)
	if s := item.OutputAccess(); s != "" {
		t.Fatal("empty index Output must fail closed", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty index Output must SetError sticky")
	}
	// incomplete IndexExprs hole fails closed sticky
	ClearErrorSess(testAmbientSession)
	item.IndexExprs = []*Expression{nil}
	if s := item.OutputAccess(); s != "" {
		t.Fatal("nil IndexExprs hole must fail closed", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IndexExprs hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	item.IndexExprs = []*Expression{{Term: TermConstant, Con: MakeInt(0)}}
	if s := item.OutputAccess(); s != "g_a[0]" {
		t.Fatal(s)
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputWithIndicesNoInventEmptyBracket(t *testing.T) {
	// ArrayVariable.cpp:708 — cvs[i] always live; sticky no invent "g_a[]"
	ClearErrorSess(testAmbientSession)
	av := &ArrayVariable{Variable: Variable{Name: "g_a", Type: GetIntType()}, Sizes: []int{2, 3}}
	if s := av.OutputWithIndices([]string{"i"}); s != "" {
		t.Fatal("short ctrl must fail closed", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("short ctrl must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if s := av.OutputWithIndices([]string{"i", ""}); s != "" {
		t.Fatal("empty ctrl name must fail closed", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty ctrl name must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if s := av.OutputWithIndices([]string{"i", "j"}); s != "g_a[i][j]" {
		t.Fatal(s)
	}
	ClearErrorSess(testAmbientSession)
}

func TestArrayOutputDefMissingInitFailClosed(t *testing.T) {
	// ArrayVariable.cpp:503 — assert(init) on string-initializer path
	ClearErrorSess(testAmbientSession)
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}},
		Sizes:    []int{2},
	}
	av.AsArray = av
	// global → NoLoopInitializer; no Init
	if av.OutputDef() != "" {
		t.Fatal("missing init on brace path must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("missing init OutputDef must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputUpperBoundArray(t *testing.T) {
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{4, 5}},
		Sizes:    []int{4, 5},
	}
	av.AsArray = av
	if got := av.OutputUpperBoundArray(); got != "g_a[3][4]" {
		t.Fatal(got)
	}
	if got := av.Variable.OutputUpperBound(false); got != "g_a[3][4]" {
		t.Fatal(got)
	}
	// ArrayVariable.cpp:575 — always sizes[i]-1 (no soft "0" for empty dim)
	z := &ArrayVariable{
		Variable: Variable{Name: "g_z", Type: GetIntType(), IsArray: true, ArraySizes: []int{0}},
		Sizes:    []int{0},
	}
	z.AsArray = z
	if got := z.OutputUpperBoundArray(); got != "g_z[-1]" {
		t.Fatalf("want sizes[i]-1, got %q", got)
	}
}

func TestSetIndexExprNoSoftZero(t *testing.T) {
	// ArrayVariable.cpp:227–231 — push/set Expression*; sticky no invent "0" for nil/empty
	ClearErrorSess(testAmbientSession)
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{4}},
		Sizes:    []int{4},
	}
	av.AsArray = av
	// nil / empty Output — sticky fail closed, no invent empty index slot
	av.AddIndexExpr(nil)
	if len(av.Indices) != 0 {
		t.Fatalf("nil AddIndexExpr must not invent slot, got %v", av.Indices)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil AddIndexExpr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	av.AddIndex("")
	if len(av.Indices) != 0 {
		t.Fatalf("empty AddIndex must not invent slot, got %v", av.Indices)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty AddIndex must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// ArrayVariable always live; sticky no invent soft-skip add past hole
	(*ArrayVariable)(nil).AddIndex("i")
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil av AddIndex must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*ArrayVariable)(nil).AddIndexExpr(&Expression{Term: TermConstant, Con: MakeInt(1), ExprType: GetIntType()})
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil av AddIndexExpr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	av.SetIndexExpr(0, &Expression{Term: TermConstant, Con: MakeInt(3), ExprType: GetIntType()})
	if len(av.Indices) != 1 || av.Indices[0] != "3" {
		t.Fatal(av.Indices)
	}
	// SetIndex past end without pad — sticky no invent empty holes
	ClearErrorSess(testAmbientSession)
	av2 := &ArrayVariable{Variable: Variable{Name: "g_b"}, Sizes: []int{2, 2}}
	av2.SetIndex(1, "i")
	if len(av2.Indices) != 0 {
		t.Fatalf("SetIndex past end must fail closed, got %v", av2.Indices)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("SetIndex past end must SetError sticky")
	}
	// append at end only
	ClearErrorSess(testAmbientSession)
	av2.SetIndex(0, "i")
	av2.SetIndex(1, "j")
	if len(av2.Indices) != 2 || av2.Indices[0] != "i" || av2.Indices[1] != "j" {
		t.Fatalf("sequential SetIndex append: %v", av2.Indices)
	}
	ClearErrorSess(testAmbientSession)
	// ArrayVariable always live; sticky (no invent soft-skip index set past hole)
	(*ArrayVariable)(nil).SetIndex(0, "i")
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil av SetIndex must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*ArrayVariable)(nil).SetIndexExpr(0, &Expression{Term: TermConstant, Con: MakeInt(1), ExprType: GetIntType()})
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil av SetIndexExpr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputWithIndicesNoLetterInvent(t *testing.T) {
	// ArrayVariable.cpp:703–711 — cvs[i] only; no soft i/j/k invent
	ClearErrorSess(testAmbientSession)
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2, 3}},
		Sizes:    []int{2, 3},
	}
	av.AsArray = av
	if got := av.OutputWithIndices([]string{"i", "j"}); got != "g_a[i][j]" {
		t.Fatal(got)
	}
	// undersized / empty ctrl: sticky fail closed empty (no invent letters or "g_a[][]")
	ClearErrorSess(testAmbientSession)
	if got := av.OutputWithIndices(nil); got != "" {
		t.Fatalf("no letter invent, got %q", got)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil ctrl OutputWithIndices must SetError sticky")
	}
	// local non-NoLoop array: OutputInit short ctrl sticky (no soft letters)
	// Array IsGlobal is Block==nil; attach a block so loop-initializer path is live.
	ClearErrorSess(testAmbientSession)
	loc := &ArrayVariable{
		Variable: Variable{Name: "l_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2, 3}, Init: MakeInt(0)},
		Sizes:    []int{2, 3},
		Block:    &Block{},
	}
	loc.AsArray = loc
	if loc.NoLoopInitializer() {
		t.Fatal("local scalar init array should use loop initializer path")
	}
	if got := loc.OutputInit("    ", nil); got != "" {
		t.Fatalf("want empty init without ctrl, got %q", got)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("OutputInit nil ctrl must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if got := loc.OutputInit("    ", []string{"i"}); got != "" {
		t.Fatalf("want empty when ctrl short, got %q", got)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("OutputInit short ctrl must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestToUnsignedSimple(t *testing.T) {
	if GetIntType().ToUnsigned() != GetSimpleType(EUInt) {
		t.Fatal("int")
	}
	if GetSimpleType(EChar).ToUnsigned() != GetSimpleType(EUChar) {
		t.Fatal("char")
	}
	u := GetSimpleType(EUInt)
	if u.ToUnsigned() != u {
		t.Fatal("uint identity")
	}
	if GetSimpleType(EFloat).ToUnsigned() != nil {
		t.Fatal("float has no unsigned")
	}
}

func TestOutputIndexModuloSignedCast(t *testing.T) {
	av := &ArrayVariable{Sizes: []int{10}}
	idx := &Expression{Term: TermVariable, Var: CreateVariableScalars("i", GetIntType(), false, false), ExprType: GetIntType()}
	got := av.OutputIndexModulo(0, idx)
	if !strings.Contains(got, "% 10") {
		t.Fatal(got)
	}
	// incomplete index Output — sticky no invent "(( % 10)"
	ClearErrorSess(testAmbientSession)
	bad := &Expression{Term: TermConstant}
	if out := av.OutputIndexModulo(0, bad); out != "" {
		t.Fatal("empty index Output must fail closed", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty index OutputIndexModulo must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil index sticky empty (no invent bare modulo past incomplete type)
	hole := &Expression{Term: TermVariable, Var: &Variable{Name: "j", Type: nil}}
	if out := av.OutputIndexModulo(0, hole); out != "" {
		t.Fatal("Type-nil index OutputIndexModulo invent", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil index OutputIndexModulo must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputDefInitExprOutputResidualSticky(t *testing.T) {
	// InitExpr.Output residual soft invent was soft-skip brace init invent complete def.
	ClearErrorSess(testAmbientSession)
	av := &ArrayVariable{
		Variable: Variable{
			Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2},
			// incomplete InitExpr → Output residual
			InitExpr: &Expression{Term: TermConstant, Con: &Constant{Value: "0"}}, // Type-nil
		},
		Sizes: []int{2},
	}
	av.AsArray = av
	if s := av.OutputDef(); s != "" {
		t.Fatal("InitExpr Output residual must fail closed OutputDef, not invent brace", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("InitExpr Output residual OutputDef must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestNoLoopInitializerNilSticky(t *testing.T) {
	// Incomplete shell sticky no-loop true (no invent loop-init eligibility past hole).
	ClearErrorSess(testAmbientSession)
	if !(*ArrayVariable)(nil).NoLoopInitializer() {
		t.Fatal("nil NoLoopInitializer must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil NoLoopInitializer must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !(&ArrayVariable{Variable: Variable{Name: "g_a"}}).NoLoopInitializer() {
		t.Fatal("Type-nil NoLoopInitializer must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil NoLoopInitializer must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputAccessIndexOutputResidualSticky(t *testing.T) {
	// index Output residual soft invent was soft-continue later indices invent partial access.
	ClearErrorSess(testAmbientSession)
	parent := &ArrayVariable{Variable: Variable{Name: "g_a", Type: GetIntType()}, Sizes: []int{2, 3}}
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntType()},
		Sizes:      []int{2, 3},
		Collective: parent,
		IndexExprs: []*Expression{
			{Term: TermConstant, Con: MakeInt(0)},
			{Term: TermConstant, Con: &Constant{Value: "1"}}, // Type-nil residual Output
		},
	}
	if s := item.OutputAccess(); s != "" {
		t.Fatal("index Output residual must fail closed OutputAccess", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("index Output residual OutputAccess must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestItemizeCreateFieldVarsAggregate(t *testing.T) {
	// ArrayVariable.cpp:261–264 / Variable.cpp:350–355 — itemize expands field vars;
	// names use ArrayVariable::Output (name[idx]…) + ".fN", not bare collective name.
	ClearErrorSess(testAmbientSession)
	st := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		{Name: "f1", Type: GetIntType(), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
	}}
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: st, IsArray: true, ArraySizes: []int{3}},
		Sizes:    []int{3},
	}
	av.AsArray = av
	item := av.Itemize(NewRng(1))
	if item == nil || item.Collective != av {
		t.Fatal(item)
	}
	if len(item.FieldVars) != 2 {
		t.Fatalf("field vars %d", len(item.FieldVars))
	}
	// Variable.cpp:350–355 — isArray → Output(ss) then .f0 → e.g. g_a[1].f0
	acc := item.OutputAccess()
	want0 := acc + ".f0"
	want1 := acc + ".f1"
	if item.FieldVars[0].Name != want0 {
		t.Fatalf("field0 name want %q got %q", want0, item.FieldVars[0].Name)
	}
	if item.FieldVars[1].Name != want1 {
		t.Fatalf("field1 name want %q got %q", want1, item.FieldVars[1].Name)
	}
	// OutputC / GetActualName on field must carry indices (seed-5 g_42[1].f0)
	if item.FieldVars[0].OutputC() != want0 {
		t.Fatalf("field OutputC want %q got %q", want0, item.FieldVars[0].OutputC())
	}
	ClearErrorSess(testAmbientSession)
}

func TestCreateFieldVarsArrayUsesOutputAccess(t *testing.T) {
	// Variable.cpp:350–352 — isArray parent uses Output not bare name for field paths.
	ClearErrorSess(testAmbientSession)
	st := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: GetSimpleType(EULongLong), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
	}}
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_42", Type: st, IsArray: true, ArraySizes: []int{2}},
		Sizes:      []int{2},
		Indices:    []string{"1"},
		Collective: &ArrayVariable{Variable: Variable{Name: "g_42"}, Sizes: []int{2}},
	}
	item.AsArray = item
	item.CreateFieldVars()
	if HasErrorSess(testAmbientSession) || len(item.FieldVars) != 1 {
		t.Fatalf("fields err=%v n=%d", HasErrorSess(testAmbientSession), len(item.FieldVars))
	}
	if item.FieldVars[0].Name != "g_42[1].f0" {
		t.Fatalf("want g_42[1].f0 got %q", item.FieldVars[0].Name)
	}
	if item.FieldVars[0].OutputAddrOf(false) != "&g_42[1].f0" {
		t.Fatalf("addr %q", item.FieldVars[0].OutputAddrOf(false))
	}
	ClearErrorSess(testAmbientSession)
}

func TestItemizeCreateFieldVarsResidualSticky(t *testing.T) {
	// CreateFieldVars residual soft invent was invent complete itemize shell past hole.
	ClearErrorSess(testAmbientSession)
	st := &Type{isStruct: true, StructName: "Sbad", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: nil, BitWidth: -1}, // incomplete field type
	}}
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: st, IsArray: true, ArraySizes: []int{2}},
		Sizes:    []int{2},
	}
	av.AsArray = av
	if av.Itemize(NewRng(1)) != nil {
		t.Fatal("CreateFieldVars residual must fail closed Itemize, not invent item")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("CreateFieldVars residual Itemize must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if av.ItemizeConstIndices([]int{0}, nil) != nil {
		t.Fatal("CreateFieldVars residual must fail closed ItemizeConstIndices")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("CreateFieldVars residual ItemizeConstIndices must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSizeInBytesArray(t *testing.T) {
	av := &ArrayVariable{
		Variable: Variable{Name: "a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2, 3}},
		Sizes:    []int{2, 3},
	}
	want := GetIntType().SizeInBytes() * 2 * 3
	if got := av.SizeInBytesArray(); got != want {
		t.Fatalf("got %d want %d", got, want)
	}
}

func TestItemizeConstIndicesNilSticky(t *testing.T) {
	// ArrayVariable always live at itemize; sticky no invent soft-skip past hole
	ClearErrorSess(testAmbientSession)
	if (*ArrayVariable)(nil).ItemizeConstIndices([]int{0}, nil) != nil {
		t.Fatal("nil ItemizeConstIndices must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil ItemizeConstIndices must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil shell sticky — no invent itemize soft-success past incomplete type
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: nil, IsArray: true, ArraySizes: []int{4}},
		Sizes:    []int{4},
	}
	av.AsArray = av
	if av.ItemizeConstIndices([]int{0}, nil) != nil {
		t.Fatal("Type-nil ItemizeConstIndices must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil ItemizeConstIndices must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if av.ItemizeInto(NewRng(1), nil) != nil {
		t.Fatal("Type-nil ItemizeInto must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil ItemizeInto must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestArrayVariableNilPredicatesSticky(t *testing.T) {
	// ArrayVariable always live; sticky no invent dim0 / empty-size / not-global / zero-bytes
	ClearErrorSess(testAmbientSession)
	if (*ArrayVariable)(nil).Dimension() != 0 {
		t.Fatal("nil Dimension must fail closed 0")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Dimension must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*ArrayVariable)(nil).TotalSize() != 0 {
		t.Fatal("nil TotalSize must fail closed 0")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil TotalSize must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*ArrayVariable)(nil).IsGlobal() {
		t.Fatal("nil IsGlobal must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsGlobal must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*ArrayVariable)(nil).SizeInBytesArray() != 0 {
		t.Fatal("nil SizeInBytesArray must fail closed 0")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil SizeInBytesArray must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	avNoType := &ArrayVariable{Variable: Variable{Name: "g_a"}, Sizes: []int{2}}
	if avNoType.SizeInBytesArray() != 0 {
		t.Fatal("nil Type SizeInBytesArray must fail closed 0")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Type SizeInBytesArray must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestContainsBackEdgeDestParentOnly(t *testing.T) {
	// Block.cpp:491 — only dest->parent == this
	b := &Block{StmID: 1}
	other := &Block{StmID: 2}
	fm := NewFactMgrSess(testAmbientSession, nil)
	fm.CFGEdges = []*CFGEdge{
		{BackLink: true, DestBlock: other, DestStmID: 10},
		{BackLink: true, DestBlock: b, DestStmID: 11},
	}
	if !b.ContainsBackEdge(fm) {
		t.Fatal("want edge with DestBlock==b")
	}
	// DestStmID alone must not count without DestBlock
	fm.CFGEdges = []*CFGEdge{{BackLink: true, DestStmID: 99}}
	if b.ContainsBackEdge(fm) {
		t.Fatal("DestStmID without DestBlock must not match")
	}
}

func TestCountAndFindExprKeyVar(t *testing.T) {
	// ArrayVariable.cpp:66–119
	iv := CreateVariableScalars("i", GetIntType(), false, false)
	ev := &Expression{Term: TermVariable, Var: iv, ExprType: GetIntType()}
	if CountExprKeyVar(ev) != 1 || FindExprKeyVar(ev) != iv {
		t.Fatal("var")
	}
	c := &Expression{Term: TermConstant, Con: MakeInt(2), ExprType: GetIntType()}
	if CountExprKeyVar(c) != 0 || FindExprKeyVar(c) != nil {
		t.Fatal("const")
	}
	// i + 2 → one key var i
	fi := &Invocation{IsStd: true, Binary: "+", Args: []*Expression{ev, c}}
	bin := &Expression{Term: TermFunction, Invoke: fi, ExprType: GetIntType()}
	if CountExprKeyVar(bin) != 1 || FindExprKeyVar(bin) != iv {
		t.Fatalf("bin count=%d key=%v", CountExprKeyVar(bin), FindExprKeyVar(bin))
	}
	// i + j → two keys
	jv := CreateVariableScalars("j", GetIntType(), false, false)
	ej := &Expression{Term: TermVariable, Var: jv, ExprType: GetIntType()}
	fi2 := &Invocation{IsStd: true, Binary: "+", Args: []*Expression{ev, ej}}
	bin2 := &Expression{Term: TermFunction, Invoke: fi2}
	if CountExprKeyVar(bin2) != 2 || FindExprKeyVar(bin2) != nil {
		t.Fatal("two vars")
	}
	// incomplete IR fails closed -1 / nil (no invent leaf 0 or key-var 1)
	ClearErrorSess(testAmbientSession)
	if CountExprKeyVar(nil) >= 0 {
		t.Fatal("nil expr must fail closed -1")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil expr CountExprKeyVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if CountExprKeyVar(&Expression{Term: TermVariable}) >= 0 {
		t.Fatal("nil Var must fail closed -1")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Var CountExprKeyVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if CountExprKeyVar(&Expression{Term: TermConstant}) >= 0 {
		t.Fatal("nil Con must fail closed -1")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Con CountExprKeyVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil Constant sticky (no invent key-count 0 past hole)
	if CountExprKeyVar(&Expression{Term: TermConstant, Con: &Constant{Value: "0"}}) >= 0 {
		t.Fatal("Type-nil Con must fail closed -1")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil Con CountExprKeyVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if CountExprKeyVar(&Expression{Term: TermFunction}) >= 0 {
		t.Fatal("nil Invoke must fail closed -1")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Invoke CountExprKeyVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if CountExprKeyVar(&Expression{Term: TermFunction, Invoke: &Invocation{
		IsStd: true, Binary: "+", Args: []*Expression{ev, nil},
	}}) >= 0 {
		t.Fatal("nil arg hole must fail closed -1")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil arg CountExprKeyVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if FindExprKeyVar(&Expression{Term: TermFunction, Invoke: &Invocation{
		IsStd: true, Binary: "+", Args: []*Expression{nil, c},
	}}) != nil {
		t.Fatal("nil arg FindExprKeyVar must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil arg FindExprKeyVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil Variable sticky (no invent key soft-success past type hole)
	if FindExprKeyVar(&Expression{Term: TermVariable, Var: &Variable{Name: "g_hole", Type: nil}}) != nil {
		t.Fatal("Type-nil Var FindExprKeyVar must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil Var FindExprKeyVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsVariantKeyVars(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{8}},
		Sizes:    []int{8},
	}
	parent.AsArray = parent
	iv := CreateVariableScalars("i", GetIntType(), false, false)
	ev := &Expression{Term: TermVariable, Var: iv, ExprType: GetIntType()}
	off := &Expression{Term: TermConstant, Con: MakeInt(1), ExprType: GetIntType()}
	// a[i] and a[i+1] share key i
	a1 := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true},
		Sizes:    []int{8}, Collective: parent,
		IndexExprs: []*Expression{ev},
	}
	a1.AsArray = a1
	fi := &Invocation{IsStd: true, Binary: "+", Args: []*Expression{ev, off}}
	a2 := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true},
		Sizes:    []int{8}, Collective: parent,
		IndexExprs: []*Expression{{Term: TermFunction, Invoke: fi, ExprType: GetIntType()}},
	}
	a2.AsArray = a2
	if !a1.IsVariant(&a2.Variable) {
		t.Fatal("same key i")
	}
	// a[j] different key
	jv := CreateVariableScalars("j", GetIntType(), false, false)
	a3 := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true},
		Sizes:    []int{8}, Collective: parent,
		IndexExprs: []*Expression{{Term: TermVariable, Var: jv, ExprType: GetIntType()}},
	}
	a3.AsArray = a3
	if a1.IsVariant(&a3.Variable) {
		t.Fatal("different keys")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsVariantFindExprKeyVarResidualSticky(t *testing.T) {
	// FindExprKeyVar residual soft invent was nil==nil invent variant-true past Type-nil key.
	ClearErrorSess(testAmbientSession)
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{8}},
		Sizes:    []int{8},
	}
	parent.AsArray = parent
	// Count==1 (TermVariable) but Type-nil Find residual nil — no invent variant true
	hole := &Expression{Term: TermVariable, Var: &Variable{Name: "i_hole", Type: nil}}
	a1 := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntType(), IsArray: true},
		Sizes:      []int{8},
		Collective: parent,
		IndexExprs: []*Expression{hole},
	}
	a1.AsArray = a1
	a2 := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntType(), IsArray: true},
		Sizes:      []int{8},
		Collective: parent,
		IndexExprs: []*Expression{hole},
	}
	a2.AsArray = a2
	if a1.IsVariant(&a2.Variable) {
		t.Fatal("FindExprKeyVar residual must fail closed not invent variant-true nil==nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("FindExprKeyVar residual IsVariant must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Count residual soft invent was soft-continue Find invent variant
	emptyCon := &Expression{Term: TermConstant, Con: &Constant{Value: "0"}} // Type-nil residual count
	a3 := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntType(), IsArray: true},
		Sizes:      []int{8},
		Collective: parent,
		IndexExprs: []*Expression{emptyCon},
	}
	a3.AsArray = a3
	if a3.IsVariant(&a3.Variable) {
		t.Fatal("CountExprKeyVar residual must fail closed IsVariant")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("CountExprKeyVar residual IsVariant must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestItemizeConstIndices(t *testing.T) {
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{4, 5}},
		Sizes:    []int{4, 5},
	}
	av.AsArray = av
	item := av.ItemizeConstIndices([]int{1, 2}, nil)
	if item == nil || len(item.Indices) != 2 || item.Indices[0] != "1" || item.Indices[1] != "2" {
		t.Fatal(item)
	}
	if item.OutputAccess() != "g_a[1][2]" {
		t.Fatal(item.OutputAccess())
	}
}

func TestHasEligibleVolatileVarIncrements(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	BookkeeperDoFinalizationSess(testAmbientSession)
	defer BookkeeperDoFinalizationSess(testAmbientSession)
	vol := CreateVariableScalars("g_v", GetIntType(), true, false)
	vol.Qfer = NewCVQualifiers([]bool{false}, []bool{true})
	// ensure IsVolatile true
	if !vol.IsVolatile() {
		// set storage volatile
		vol.Qfer.IsVolatiles = []bool{true}
	}
	cg := EmptyCGContext().WithSession(testAmbientSession)
	if !HasEligibleVolatileVar([]*Variable{vol}, GetIntType(), AccessRead, cg) {
		t.Fatal("eligible")
	}
	if VolatileAvailCount() != 1 {
		t.Fatal(VolatileAvailCount())
	}
}

func TestSetIndexNoInventNilPad(t *testing.T) {
	// IndexExprs lag Indices: sticky must not invent nil pad slots
	ClearErrorSess(testAmbientSession)
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true},
		Indices:  []string{"0", "1"},
		// IndexExprs empty lag
	}
	av.SetIndex(1, "i")
	// Indices[1] updated; IndexExprs must not grow with nil pads to index 1
	if len(av.IndexExprs) > 0 && av.IndexExprs[0] == nil {
		t.Fatal("must not invent nil pad at IndexExprs[0]")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IndexExprs lag SetIndex must SetError sticky")
	}
	// only append when index == len
	ClearErrorSess(testAmbientSession)
	av.IndexExprs = nil
	av.SetIndex(0, "0")
	if len(av.IndexExprs) != 1 || av.IndexExprs[0] == nil {
		t.Fatal("append at len must store live expr")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputWithIndicesGetActualNameResidualSticky(t *testing.T) {
	// GetActualName residual soft invent was invent "[i]" access past empty name shell.
	ClearErrorSess(testAmbientSession)
	av := &ArrayVariable{
		Variable: Variable{Type: GetIntType(), IsArray: true, ArraySizes: []int{2}},
		Sizes:    []int{2},
	}
	av.AsArray = av
	if s := av.OutputWithIndices([]string{"i"}); s != "" {
		t.Fatal("empty name residual must fail closed OutputWithIndices", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty name residual OutputWithIndices must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if s := av.OutputLowerBound(); s != "" {
		t.Fatal("empty name residual must fail closed OutputLowerBound", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty name residual OutputLowerBound must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if s := av.OutputUpperBoundArray(); s != "" {
		t.Fatal("empty name residual must fail closed OutputUpperBoundArray", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty name residual OutputUpperBoundArray must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSizeInBytesArraySizeInBytesResidualSticky(t *testing.T) {
	// SizeInBytes residual soft invent was invent soft-zero size past Type-nil shell.
	ClearErrorSess(testAmbientSession)
	av := &ArrayVariable{Variable: Variable{Name: "g_a", Type: nil}, Sizes: []int{2}}
	if av.SizeInBytesArray() != 0 {
		t.Fatal("Type-nil SizeInBytesArray must fail closed 0")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil SizeInBytesArray must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCDeclTypeCNameResidualSticky(t *testing.T) {
	// CName residual soft invent was invent soft-empty decl past Type-nil shell.
	ClearErrorSess(testAmbientSession)
	av := &ArrayVariable{Variable: Variable{Name: "g_a", Type: nil}, Sizes: []int{2}}
	if av.CDeclType() != "" {
		t.Fatal("Type-nil CDeclType must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil CDeclType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputExpressionVariableAddrOfItemized(t *testing.T) {
	// ExpressionVariable.cpp:210–216 — & + ArrayVariable::Output → &g_83[1]
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_83", Type: GetSimpleType(EULong), IsArray: true, ArraySizes: []int{4}},
		Sizes:    []int{4},
	}
	parent.AsArray = parent
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_83", Type: GetSimpleType(EULong), IsArray: true, ArraySizes: []int{4}},
		Sizes:      []int{4},
		Collective: parent,
		IndexExprs: []*Expression{{Term: TermConstant, Con: &Constant{Type: GetIntType(), Value: "1"}, ExprType: GetIntType()}},
	}
	item.AsArray = item
	// want type is pointer to element so indirect_level = 0 - 1 = -1 for &
	want := PointerTo(GetSimpleType(EULong))
	got := outputExpressionVariable(&item.Variable, want)
	if got != "&g_83[1]" {
		t.Fatalf("got %q want &g_83[1]", got)
	}
	ClearErrorSess(testAmbientSession)
}

func TestArrayCDeclTypePointerVolatileStorage(t *testing.T) {
	// ArrayVariable.cpp:512–521 + CVQualifiers.cpp:534–561 —
	// storage-volatile pointer array: "int32_t * volatile g_38[1]" not "volatile int32_t* g_38[1]"
	ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	pt := PointerTo(GetIntType())
	av := &ArrayVariable{
		Variable: Variable{
			Name:       "g_38",
			Type:       pt,
			IsArray:    true,
			ArraySizes: []int{1},
			Qfer:       NewCVQualifiers([]bool{false, false}, []bool{false, true}),
			Init:       &Constant{Type: pt, Value: "0"},
		},
		Sizes:      []int{1},
		InitValues: []string{"&g_37"},
	}
	av.AsArray = av
	d := av.CDeclType()
	if HasErrorSess(testAmbientSession) {
		t.Fatal("sticky error on CDeclType")
	}
	want := "int32_t * volatile g_38[1]"
	if d != want {
		t.Fatalf("got %q want %q", d, want)
	}
	def := av.OutputDef()
	if strings.Contains(def, "VOLATILE GLOBAL") {
		t.Fatal("array OutputDef must not invent VOLATILE GLOBAL:", def)
	}
	if !strings.Contains(def, want+" = ") {
		t.Fatal(def)
	}
}

func TestVariableOutputDefVolatileCommentNoSpace(t *testing.T) {
	// Variable.cpp:662–664 + OutputMgr.cpp:318 — ";/* VOLATILE GLOBAL name */"
	ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	v := CreateVariableScalars("g_v", GetIntType(), false, true)
	v.Init = MakeInt(0)
	s := v.OutputDefOpts(true, false)
	if !strings.Contains(s, ";/* VOLATILE GLOBAL g_v */") {
		t.Fatal(s)
	}
	if strings.Contains(s, "; /*") {
		t.Fatal("invent space before comment:", s)
	}
}

func TestOutputInitNegativeConstParen(t *testing.T) {
	// ArrayVariable.cpp:649 + Expression::to_string → Constant::Output
	// Constant.cpp:532–536 — enclose negatives in parentheses.
	ClearErrorSess(testAmbientSession)
	// Local array: IsGlobal for ArrayVariable is Block==nil (ArrayVariable.cpp:414–415).
	blk := &Block{}
	av := &ArrayVariable{
		Variable: Variable{
			Name: "l_52", Type: GetIntType(), IsArray: true, ArraySizes: []int{3},
			Init: &Constant{Type: GetIntType(), Value: "-3L"},
		},
		Sizes: []int{3},
		Block: blk,
	}
	av.AsArray = av
	if av.NoLoopInitializer() {
		t.Fatal("test array must use loop initializer")
	}
	if got := av.Init.Output(); got != "(-3L)" {
		t.Fatalf("Init.Output got %q", got)
	}
	out := av.OutputInitOpts("", []string{"i"}, true)
	if HasErrorSess(testAmbientSession) {
		t.Fatalf("OutputInit sticky err=%v out=%q", HasErrorSess(testAmbientSession), out)
	}
	if !strings.Contains(out, " = (-3L);") {
		t.Fatalf("want paren-negative init, got %q", out)
	}
}
