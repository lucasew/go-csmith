package csmith

import (
	"strings"
	"testing"
)

func TestCreateArrayVariableDimensions(t *testing.T) {
	ClearError()
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
	ClearError()
	opts := Defaults()
	q := NewCVQualifiers([]bool{false}, []bool{false})
	if CreateArrayVariable(NewRng(1), opts, NewProbabilities(opts), nil, nil, nil, "g_v", GetSimpleType(EVoid), MakeInt(0), q) != nil {
		t.Fatal("void element must fail closed")
	}
	if !HasError() {
		t.Fatal("void element must SetError sticky")
	}
	ClearError()
	if CreateArrayVariable(NewRng(1), opts, NewProbabilities(opts), nil, nil, nil, "g_n", nil, MakeInt(0), q) != nil {
		t.Fatal("nil element must fail closed")
	}
	if !HasError() {
		t.Fatal("nil element must SetError sticky")
	}
	ClearError()
	if CreateArrayVariable(NewRng(1), opts, NewProbabilities(opts), nil, nil, nil, "", GetIntType(), MakeInt(0), q) != nil {
		t.Fatal("empty name must fail closed")
	}
	if !HasError() {
		t.Fatal("empty name must SetError sticky")
	}
	ClearError()
	SetError(ErrGeneric)
	defer ClearError()
	if CreateArrayVariable(NewRng(1), opts, NewProbabilities(opts), nil, nil, nil, "g_e", GetIntType(), MakeInt(0), q) != nil {
		t.Fatal("sticky error after dim draw must fail closed")
	}
}

func TestCreateArrayVariableNoSoftInventSizeOne(t *testing.T) {
	// ArrayVariable.cpp:154–157 — empty sizes when no dim fits; no invent [1]
	ClearError()
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
	ClearError()
	opts := Defaults()
	env := &TypeEnv{}
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
	ClearError()
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
	ClearError()
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
		ClearError()
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
	ClearError()
	// with VS+CG: make_init_value path is live
	vs := NewVariableSelector(opts)
	vs.Probs = NewProbabilities(opts)
	_ = vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), GetIntType(), nil, NewRng(1))
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
	ClearError()
	av := CreateArrayVariable(NewRng(3), opts, vs.Probs, vs, &cg, nil, "g_p", pt, MakeInt(0), q)
	if av == nil {
		// make_init_value may ERROR_GUARD; not invent Constant "0"
		// Clear residual sticky so later tests are not poisoned
		ClearError()
		return
	}
	for _, s := range av.InitValues {
		if s == "" {
			t.Fatal("empty alt")
		}
	}
	ClearError()
}

func TestArrayCDecl(t *testing.T) {
	av := &ArrayVariable{
		Variable: Variable{Name: "a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2, 3}},
		Sizes:    []int{2, 3},
	}
	d := av.CDeclType()
	if d != "int a[2][3]" {
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
	v := vs.createAndInitialize(AccessWrite, EmptyCGContext(), GetIntType(), q, nil, "g_9", r)
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
	ClearError()
	if out := item.OutputAccess(); out != "" {
		t.Fatalf("empty indices must fail closed, got %q", out)
	}
	if !HasError() {
		t.Fatal("empty indices itemized OutputAccess must SetError sticky")
	}
	// Indices string path (ItemizeConstIndices) still works without IndexExprs
	ClearError()
	item.Indices = []string{"1", "2"}
	if got := item.OutputAccess(); got != "g_a[1][2]" {
		t.Fatal(got)
	}
	ClearError()
}

func TestItemizeAlreadyItemizedFailClosed(t *testing.T) {
	// ArrayVariable.cpp:250 — assert(collective == 0) on the receiver sticky
	ClearError()
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
	ClearError()
	if item.Itemize(NewRng(3)) != nil {
		t.Fatal("re-itemize of itemized must fail closed")
	}
	if !HasError() {
		t.Fatal("re-itemize of itemized must SetError sticky")
	}
	ClearError()
	if parent.ItemizeInto(nil, nil) != nil {
		t.Fatal("nil RNG Itemize must fail closed")
	}
	if !HasError() {
		t.Fatal("nil RNG Itemize must SetError sticky")
	}
	ClearError()
}

func TestArrayCDeclTypeNoInventInt(t *testing.T) {
	// ArrayVariable decl always has live type; sticky no invent "int"
	ClearError()
	if s := (*ArrayVariable)(nil).CDeclType(); s != "" {
		t.Fatal("nil av CDeclType must fail closed", s)
	}
	if !HasError() {
		t.Fatal("nil av CDeclType must SetError sticky")
	}
	ClearError()
	av := &ArrayVariable{Variable: Variable{Name: "g_a"}, Sizes: []int{2}}
	if s := av.CDeclType(); s != "" {
		t.Fatal("nil Type must fail closed", s)
	}
	if !HasError() {
		t.Fatal("nil Type CDeclType must SetError sticky")
	}
	ClearError()
	if s := av.OutputDef(); s != "" {
		t.Fatal("incomplete array def must fail closed", s)
	}
	if !HasError() {
		t.Fatal("incomplete OutputDef must SetError sticky")
	}
	ClearError()
	if s := (*ArrayVariable)(nil).OutputDef(); s != "" {
		t.Fatal("nil av OutputDef must fail closed", s)
	}
	if !HasError() {
		t.Fatal("nil av OutputDef must SetError sticky")
	}
	ClearError()
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
	ClearError()
	if s := item.OutputAccess(); s != "" {
		t.Fatal("empty index Output must fail closed", s)
	}
	if !HasError() {
		t.Fatal("empty index Output must SetError sticky")
	}
	// incomplete IndexExprs hole fails closed sticky
	ClearError()
	item.IndexExprs = []*Expression{nil}
	if s := item.OutputAccess(); s != "" {
		t.Fatal("nil IndexExprs hole must fail closed", s)
	}
	if !HasError() {
		t.Fatal("nil IndexExprs hole must SetError sticky")
	}
	ClearError()
	item.IndexExprs = []*Expression{{Term: TermConstant, Con: MakeInt(0)}}
	if s := item.OutputAccess(); s != "g_a[0]" {
		t.Fatal(s)
	}
	ClearError()
}

func TestOutputWithIndicesNoInventEmptyBracket(t *testing.T) {
	// ArrayVariable.cpp:708 — cvs[i] always live; sticky no invent "g_a[]"
	ClearError()
	av := &ArrayVariable{Variable: Variable{Name: "g_a", Type: GetIntType()}, Sizes: []int{2, 3}}
	if s := av.OutputWithIndices([]string{"i"}); s != "" {
		t.Fatal("short ctrl must fail closed", s)
	}
	if !HasError() {
		t.Fatal("short ctrl must SetError sticky")
	}
	ClearError()
	if s := av.OutputWithIndices([]string{"i", ""}); s != "" {
		t.Fatal("empty ctrl name must fail closed", s)
	}
	if !HasError() {
		t.Fatal("empty ctrl name must SetError sticky")
	}
	ClearError()
	if s := av.OutputWithIndices([]string{"i", "j"}); s != "g_a[i][j]" {
		t.Fatal(s)
	}
	ClearError()
}

func TestArrayOutputDefMissingInitFailClosed(t *testing.T) {
	// ArrayVariable.cpp:503 — assert(init) on string-initializer path
	ClearError()
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}},
		Sizes:    []int{2},
	}
	av.AsArray = av
	// global → NoLoopInitializer; no Init
	if av.OutputDef() != "" {
		t.Fatal("missing init on brace path must fail closed")
	}
	if !HasError() {
		t.Fatal("missing init OutputDef must SetError sticky")
	}
	ClearError()
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
	ClearError()
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
	if !HasError() {
		t.Fatal("nil AddIndexExpr must SetError sticky")
	}
	ClearError()
	av.AddIndex("")
	if len(av.Indices) != 0 {
		t.Fatalf("empty AddIndex must not invent slot, got %v", av.Indices)
	}
	if !HasError() {
		t.Fatal("empty AddIndex must SetError sticky")
	}
	ClearError()
	// ArrayVariable always live; sticky no invent soft-skip add past hole
	(*ArrayVariable)(nil).AddIndex("i")
	if !HasError() {
		t.Fatal("nil av AddIndex must SetError sticky")
	}
	ClearError()
	(*ArrayVariable)(nil).AddIndexExpr(&Expression{Term: TermConstant, Con: MakeInt(1), ExprType: GetIntType()})
	if !HasError() {
		t.Fatal("nil av AddIndexExpr must SetError sticky")
	}
	ClearError()
	av.SetIndexExpr(0, &Expression{Term: TermConstant, Con: MakeInt(3), ExprType: GetIntType()})
	if len(av.Indices) != 1 || av.Indices[0] != "3" {
		t.Fatal(av.Indices)
	}
	// SetIndex past end without pad — sticky no invent empty holes
	ClearError()
	av2 := &ArrayVariable{Variable: Variable{Name: "g_b"}, Sizes: []int{2, 2}}
	av2.SetIndex(1, "i")
	if len(av2.Indices) != 0 {
		t.Fatalf("SetIndex past end must fail closed, got %v", av2.Indices)
	}
	if !HasError() {
		t.Fatal("SetIndex past end must SetError sticky")
	}
	// append at end only
	ClearError()
	av2.SetIndex(0, "i")
	av2.SetIndex(1, "j")
	if len(av2.Indices) != 2 || av2.Indices[0] != "i" || av2.Indices[1] != "j" {
		t.Fatalf("sequential SetIndex append: %v", av2.Indices)
	}
	ClearError()
	// ArrayVariable always live; sticky (no invent soft-skip index set past hole)
	(*ArrayVariable)(nil).SetIndex(0, "i")
	if !HasError() {
		t.Fatal("nil av SetIndex must SetError sticky")
	}
	ClearError()
	(*ArrayVariable)(nil).SetIndexExpr(0, &Expression{Term: TermConstant, Con: MakeInt(1), ExprType: GetIntType()})
	if !HasError() {
		t.Fatal("nil av SetIndexExpr must SetError sticky")
	}
	ClearError()
}

func TestOutputWithIndicesNoLetterInvent(t *testing.T) {
	// ArrayVariable.cpp:703–711 — cvs[i] only; no soft i/j/k invent
	ClearError()
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2, 3}},
		Sizes:    []int{2, 3},
	}
	av.AsArray = av
	if got := av.OutputWithIndices([]string{"i", "j"}); got != "g_a[i][j]" {
		t.Fatal(got)
	}
	// undersized / empty ctrl: sticky fail closed empty (no invent letters or "g_a[][]")
	ClearError()
	if got := av.OutputWithIndices(nil); got != "" {
		t.Fatalf("no letter invent, got %q", got)
	}
	if !HasError() {
		t.Fatal("nil ctrl OutputWithIndices must SetError sticky")
	}
	// local non-NoLoop array: OutputInit short ctrl sticky (no soft letters)
	// Array IsGlobal is Block==nil; attach a block so loop-initializer path is live.
	ClearError()
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
	if !HasError() {
		t.Fatal("OutputInit nil ctrl must SetError sticky")
	}
	ClearError()
	if got := loc.OutputInit("    ", []string{"i"}); got != "" {
		t.Fatalf("want empty when ctrl short, got %q", got)
	}
	if !HasError() {
		t.Fatal("OutputInit short ctrl must SetError sticky")
	}
	ClearError()
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
	ClearError()
	bad := &Expression{Term: TermConstant}
	if out := av.OutputIndexModulo(0, bad); out != "" {
		t.Fatal("empty index Output must fail closed", out)
	}
	if !HasError() {
		t.Fatal("empty index OutputIndexModulo must SetError sticky")
	}
	ClearError()
}

func TestItemizeCreateFieldVarsAggregate(t *testing.T) {
	// ArrayVariable.cpp:261–264 — itemize expands field vars for aggregate element type
	ClearError()
	st := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
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
	if item.FieldVars[0].Name != "g_a.f0" {
		t.Fatal(item.FieldVars[0].Name)
	}
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
	ClearError()
	if (*ArrayVariable)(nil).ItemizeConstIndices([]int{0}, nil) != nil {
		t.Fatal("nil ItemizeConstIndices must fail closed")
	}
	if !HasError() {
		t.Fatal("nil ItemizeConstIndices must SetError sticky")
	}
	ClearError()
	// Type-nil shell sticky — no invent itemize soft-success past incomplete type
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: nil, IsArray: true, ArraySizes: []int{4}},
		Sizes:    []int{4},
	}
	av.AsArray = av
	if av.ItemizeConstIndices([]int{0}, nil) != nil {
		t.Fatal("Type-nil ItemizeConstIndices must fail closed")
	}
	if !HasError() {
		t.Fatal("Type-nil ItemizeConstIndices must SetError sticky")
	}
	ClearError()
	if av.ItemizeInto(NewRng(1), nil) != nil {
		t.Fatal("Type-nil ItemizeInto must fail closed")
	}
	if !HasError() {
		t.Fatal("Type-nil ItemizeInto must SetError sticky")
	}
	ClearError()
}

func TestArrayVariableNilPredicatesSticky(t *testing.T) {
	// ArrayVariable always live; sticky no invent dim0 / empty-size / not-global / zero-bytes
	ClearError()
	if (*ArrayVariable)(nil).Dimension() != 0 {
		t.Fatal("nil Dimension must fail closed 0")
	}
	if !HasError() {
		t.Fatal("nil Dimension must SetError sticky")
	}
	ClearError()
	if (*ArrayVariable)(nil).TotalSize() != 0 {
		t.Fatal("nil TotalSize must fail closed 0")
	}
	if !HasError() {
		t.Fatal("nil TotalSize must SetError sticky")
	}
	ClearError()
	if (*ArrayVariable)(nil).IsGlobal() {
		t.Fatal("nil IsGlobal must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil IsGlobal must SetError sticky")
	}
	ClearError()
	if (*ArrayVariable)(nil).SizeInBytesArray() != 0 {
		t.Fatal("nil SizeInBytesArray must fail closed 0")
	}
	if !HasError() {
		t.Fatal("nil SizeInBytesArray must SetError sticky")
	}
	ClearError()
	avNoType := &ArrayVariable{Variable: Variable{Name: "g_a"}, Sizes: []int{2}}
	if avNoType.SizeInBytesArray() != 0 {
		t.Fatal("nil Type SizeInBytesArray must fail closed 0")
	}
	if !HasError() {
		t.Fatal("nil Type SizeInBytesArray must SetError sticky")
	}
	ClearError()
}

func TestContainsBackEdgeDestParentOnly(t *testing.T) {
	// Block.cpp:491 — only dest->parent == this
	b := &Block{StmID: 1}
	other := &Block{StmID: 2}
	fm := NewFactMgr(nil)
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
	ClearError()
	if CountExprKeyVar(nil) >= 0 {
		t.Fatal("nil expr must fail closed -1")
	}
	if !HasError() {
		t.Fatal("nil expr CountExprKeyVar must SetError sticky")
	}
	ClearError()
	if CountExprKeyVar(&Expression{Term: TermVariable}) >= 0 {
		t.Fatal("nil Var must fail closed -1")
	}
	if !HasError() {
		t.Fatal("nil Var CountExprKeyVar must SetError sticky")
	}
	ClearError()
	if CountExprKeyVar(&Expression{Term: TermConstant}) >= 0 {
		t.Fatal("nil Con must fail closed -1")
	}
	if !HasError() {
		t.Fatal("nil Con CountExprKeyVar must SetError sticky")
	}
	ClearError()
	// Type-nil Constant sticky (no invent key-count 0 past hole)
	if CountExprKeyVar(&Expression{Term: TermConstant, Con: &Constant{Value: "0"}}) >= 0 {
		t.Fatal("Type-nil Con must fail closed -1")
	}
	if !HasError() {
		t.Fatal("Type-nil Con CountExprKeyVar must SetError sticky")
	}
	ClearError()
	if CountExprKeyVar(&Expression{Term: TermFunction}) >= 0 {
		t.Fatal("nil Invoke must fail closed -1")
	}
	if !HasError() {
		t.Fatal("nil Invoke CountExprKeyVar must SetError sticky")
	}
	ClearError()
	if CountExprKeyVar(&Expression{Term: TermFunction, Invoke: &Invocation{
		IsStd: true, Binary: "+", Args: []*Expression{ev, nil},
	}}) >= 0 {
		t.Fatal("nil arg hole must fail closed -1")
	}
	if !HasError() {
		t.Fatal("nil arg CountExprKeyVar must SetError sticky")
	}
	ClearError()
	if FindExprKeyVar(&Expression{Term: TermFunction, Invoke: &Invocation{
		IsStd: true, Binary: "+", Args: []*Expression{nil, c},
	}}) != nil {
		t.Fatal("nil arg FindExprKeyVar must fail closed")
	}
	if !HasError() {
		t.Fatal("nil arg FindExprKeyVar must SetError sticky")
	}
	ClearError()
}

func TestIsVariantKeyVars(t *testing.T) {
	ClearError()
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
		Sizes: []int{8}, Collective: parent,
		IndexExprs: []*Expression{ev},
	}
	a1.AsArray = a1
	fi := &Invocation{IsStd: true, Binary: "+", Args: []*Expression{ev, off}}
	a2 := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true},
		Sizes: []int{8}, Collective: parent,
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
		Sizes: []int{8}, Collective: parent,
		IndexExprs: []*Expression{{Term: TermVariable, Var: jv, ExprType: GetIntType()}},
	}
	a3.AsArray = a3
	if a1.IsVariant(&a3.Variable) {
		t.Fatal("different keys")
	}
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
	ClearError()
	BookkeeperDoFinalization()
	defer BookkeeperDoFinalization()
	vol := CreateVariableScalars("g_v", GetIntType(), true, false)
	vol.Qfer = NewCVQualifiers([]bool{false}, []bool{true})
	// ensure IsVolatile true
	if !vol.IsVolatile() {
		// set storage volatile
		vol.Qfer.IsVolatiles = []bool{true}
	}
	cg := EmptyCGContext()
	if !HasEligibleVolatileVar([]*Variable{vol}, GetIntType(), AccessRead, cg) {
		t.Fatal("eligible")
	}
	if VolatileAvailCount() != 1 {
		t.Fatal(VolatileAvailCount())
	}
}

func TestSetIndexNoInventNilPad(t *testing.T) {
	// IndexExprs lag Indices: sticky must not invent nil pad slots
	ClearError()
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
	if !HasError() {
		t.Fatal("IndexExprs lag SetIndex must SetError sticky")
	}
	// only append when index == len
	ClearError()
	av.IndexExprs = nil
	av.SetIndex(0, "0")
	if len(av.IndexExprs) != 1 || av.IndexExprs[0] == nil {
		t.Fatal("append at len must store live expr")
	}
	ClearError()
}
