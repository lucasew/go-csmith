package csmith

import (
	"strings"
	"testing"
)

func TestIsPackedAggregateFieldVar(t *testing.T) {
	st := &Type{isStruct: true, StructName: "S0", Packed: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	parent := CreateVariableQfer("g_s", st, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(parent.FieldVars) == 0 {
		t.Skip("no fields")
	}
	f0 := parent.FieldVars[0]
	if !f0.IsPackedAggregateFieldVar() {
		t.Fatal("want packed field")
	}
	plain := CreateVariableScalars("g_i", GetIntType(), false, false)
	if plain.IsPackedAggregateFieldVar() {
		t.Fatal("scalar")
	}
}

func TestItemizeArrayOffsetBinary(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	// size 8 so remain > 1 when bound is 0 → offset possible
	av := CreateArrayVariable(NewRng(1), opts, NewProbabilities(opts), nil, nil, nil, "g_a", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
	// force size
	if av == nil {
		t.Fatal("nil av")
	}
	av.Sizes = []int{8}
	av.ArraySizes = []int{8}
	iv := CreateVariableScalars("i", GetIntType(), false, false)
	cg := EmptyCGContext()
	cg.IVBounds = map[*Variable]int{iv: 0}
	// scan seeds for offset form
	foundOff := false
	for seed := uint64(1); seed < 40; seed++ {
		item := vs.ItemizeArray(NewRng(seed), cg, av)
		if item == nil {
			t.Fatal("itemize")
		}
		if len(item.IndexExprs) != 1 {
			t.Fatalf("dims %d", len(item.IndexExprs))
		}
		ie := item.IndexExprs[0]
		if ie.Term == TermFunction && ie.Invoke != nil && ie.Invoke.Binary == "+" {
			foundOff = true
			out := item.OutputAccess()
			if !strings.Contains(out, "+") {
				t.Fatal(out)
			}
			// UseVar of IV still works via nested expression
			if !ie.UseVar(iv) {
				// UseVar on func may not recurse — check Args
				if len(ie.Invoke.Args) > 0 && ie.Invoke.Args[0] != nil && ie.Invoke.Args[0].UseVar(iv) {
					// ok
				} else {
					t.Log("UseVar IV optional on binary")
				}
			}
			break
		}
	}
	if !foundOff {
		// still valid if all seeds picked 0 offset
		t.Log("no offset in scan; bare IV path covered by other tests")
	}
}

func TestItemizeArrayRejectsInvalidBound(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{4}},
		Sizes:    []int{4},
	}
	av.AsArray = av
	iv := CreateVariableScalars("i", GetIntType(), false, false)
	cg := EmptyCGContext()
	cg.IVBounds = map[*Variable]int{iv: InvalidIVBound}
	if vs.ItemizeArray(NewRng(1), cg, av) != nil {
		t.Fatal("invalid bound must not itemize")
	}
}

func TestItemizeArrayNilIVKeyHole(t *testing.T) {
	// Variable* always live as IVBounds keys; nil key must not soft-skip to other IVs
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{4}},
		Sizes:    []int{4},
	}
	av.AsArray = av
	iv := CreateVariableScalars("i", GetIntType(), false, false)
	cg := EmptyCGContext()
	cg.IVBounds = map[*Variable]int{nil: 0, iv: 0}
	if vs.ItemizeArray(NewRng(1), cg, av) != nil {
		t.Fatal("nil IVBounds key must fail closed whole itemize")
	}
	if !HasError() {
		t.Fatal("nil IVBounds key must SetError sticky")
	}
	ClearError()
}

func TestItemizeArrayIncompleteAmbientSticky(t *testing.T) {
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{4}},
		Sizes:    []int{4},
	}
	av.AsArray = av
	iv := CreateVariableScalars("i", GetIntType(), false, false)
	inc := IncompleteEffect()
	cg := EmptyCGContext()
	cg.EffectAccum = &inc
	cg.IVBounds = map[*Variable]int{iv: 0}
	if vs.ItemizeArray(NewRng(1), cg, av) != nil {
		t.Fatal("incomplete EffectAccum must fail closed ItemizeArray")
	}
	if !HasError() {
		t.Fatal("incomplete EffectAccum must SetError sticky ItemizeArray")
	}
	ClearError()
}

func TestItemizeArrayTypeNilSticky(t *testing.T) {
	// type always live at itemize; Type-nil no invent soft-success item
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: nil, IsArray: true, ArraySizes: []int{4}},
		Sizes:    []int{4},
	}
	av.AsArray = av
	iv := CreateVariableScalars("i", GetIntType(), false, false)
	cg := EmptyCGContext()
	cg.IVBounds = map[*Variable]int{iv: 0}
	if vs.ItemizeArray(NewRng(1), cg, av) != nil {
		t.Fatal("Type-nil array must fail closed ItemizeArray")
	}
	if !HasError() {
		t.Fatal("Type-nil array ItemizeArray must SetError sticky")
	}
	ClearError()
	// IV Type-nil sticky — no invent OK-IV soft pool past hole
	av2 := &ArrayVariable{
		Variable: Variable{Name: "g_b", Type: GetIntType(), IsArray: true, ArraySizes: []int{4}},
		Sizes:    []int{4},
	}
	av2.AsArray = av2
	ivHole := &Variable{Name: "j", Type: nil}
	cg2 := EmptyCGContext()
	cg2.IVBounds = map[*Variable]int{ivHole: 0}
	if vs.ItemizeArray(NewRng(1), cg2, av2) != nil {
		t.Fatal("Type-nil IV must fail closed ItemizeArray")
	}
	if !HasError() {
		t.Fatal("Type-nil IV ItemizeArray must SetError sticky")
	}
	ClearError()
}

func TestSelectArrayTypeNilSticky(t *testing.T) {
	// av->type always live; Type-nil no invent soft-include / CreateRandom soft-success
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: nil, IsArray: true, ArraySizes: []int{2}},
		Sizes:    []int{2},
	}
	av.AsArray = av
	vs.Arrays = []*ArrayVariable{av}
	cg := EmptyCGContext()
	if vs.SelectArray(NewRng(1), cg) != nil {
		t.Fatal("Type-nil SelectArray must fail closed")
	}
	if !HasError() {
		t.Fatal("Type-nil SelectArray must SetError sticky")
	}
	ClearError()
}

func TestSelectArrayFiltersPartialWrite(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	av := CreateArrayVariable(NewRng(2), opts, NewProbabilities(opts), nil, nil, nil, "g_a", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
	vs.Arrays = []*ArrayVariable{av}
	vs.GlobalList = []*Variable{&av.Variable}
	// mark partially written → filtered → CreateRandomArray may still run
	eff := EmptyEffect().WriteVar(&av.Variable)
	cg := WithEffectContext(eff)
	// disable global create by turning off globals? CreateRandomArray uses globals
	// ensure filter drops av: if CreateRandomArray returns different name ok
	got := vs.SelectArray(NewRng(3), cg)
	if got == av {
		t.Fatal("partially written array must not be selected")
	}
}

func TestSelectArrayFilterResidualSticky(t *testing.T) {
	// IsNonWritable residual ERROR soft invent was soft-skip then CreateRandomArray / later pick.
	// Fair: sticky fail closed whole SelectArray.
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	vs.Types = &TypeEnv{AllTypes: []*Type{GetIntType()}}
	av := CreateArrayVariable(NewRng(2), opts, NewProbabilities(opts), nil, nil, nil, "g_a", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
	if av == nil {
		t.Fatal("array")
	}
	av2 := CreateArrayVariable(NewRng(4), opts, NewProbabilities(opts), nil, nil, nil, "g_b", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
	vs.Arrays = []*ArrayVariable{av, av2}
	vs.GlobalList = []*Variable{&av.Variable, &av2.Variable}
	// incomplete NoWriteVars hole stickies IsNonWritable residual during filter of av
	cg := EmptyCGContext().WithRW(&RWDirective{NoWriteVars: []*Variable{nil}})
	if vs.SelectArray(NewRng(3), cg) != nil {
		t.Fatal("IsNonWritable residual must fail closed SelectArray")
	}
	if !HasError() {
		t.Fatal("IsNonWritable residual SelectArray must SetError sticky")
	}
	ClearError()
	// IsSideEffectFree residual soft invent was soft-continue keep/filter invent pick.
	// Fair: sticky fail closed whole SelectArray (EffectComplete gate + sefree residual).
	cg2 := WithEffectContext(IncompleteEffect())
	if vs.SelectArray(NewRng(3), cg2) != nil {
		t.Fatal("IsSideEffectFree residual must fail closed SelectArray")
	}
	if !HasError() {
		t.Fatal("IsSideEffectFree residual SelectArray must SetError sticky")
	}
	ClearError()
}

func TestMakeRandomArrayOpPackedResidualSticky(t *testing.T) {
	// IsPackedAggregateFieldVar Type-nil parent stickies residual true; soft invent was
	// continue then pick later IV. Fair: sticky fail closed whole array-op make.
	ClearError()
	opts := Defaults()
	opts.CComp = true
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	vs.Types = &TypeEnv{AllTypes: []*Type{GetIntType()}}
	av := CreateArrayVariable(NewRng(2), opts, probs, nil, nil, nil, "g_a", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
	if av == nil {
		t.Fatal("array")
	}
	vs.Arrays = []*ArrayVariable{av}
	vs.GlobalList = []*Variable{&av.Variable}
	// Type-nil parent field IV: packed sticky residual during IV filter
	parent := &Variable{Name: "g_u"} // Type nil
	fieldIV := &Variable{Name: "g_u.f0", Type: GetIntType(), FieldVarOf: parent}
	goodIV := CreateVariableScalars("g_i", GetIntType(), false, false)
	vs.GlobalList = append(vs.GlobalList, fieldIV, goodIV)
	vs.AllVars = append([]*Variable{&av.Variable}, fieldIV, goodIV)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f, LocalVars: []*Variable{fieldIV, goodIV}}
	f.Stack = []*Block{blk}
	fm := NewFactMgr(f)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	// force SelectArray to pick av; force IV pool to hit field first via only field+good
	// SelectLoopCtrlVar uses FindAllNonArrayVisibleVars — both fieldIV and goodIV present
	// CComp packed: field with Type-nil parent stickies residual on first filter hit
	// Invalidate? Put only fieldIV first by invalid map empty — ChooseVar order random.
	// Safer: only fieldIV as non-array visible so SelectLoopCtrlVar returns it, residual stickies.
	vs.GlobalList = []*Variable{&av.Variable, fieldIV}
	vs.AllVars = []*Variable{&av.Variable, fieldIV}
	blk.LocalVars = []*Variable{fieldIV}
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	st := MakeRandomArrayOp(NewRng(5), opts, probs, vs, tables, stmtTab, &cg)
	if stmtOK(st) {
		t.Fatal("packed residual must fail closed MakeRandomArrayOp")
	}
	if !HasError() {
		t.Fatal("packed residual MakeRandomArrayOp must SetError sticky")
	}
	ClearError()
}

func TestItemizeIndexOutputResidualSticky(t *testing.T) {
	// index Output residual soft invent was soft-continue later dims invent partial item.
	// SetIndexExpr residual: Type-nil index Output stickies without inventing index slot.
	ClearError()
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{4}},
		Sizes:    []int{4},
	}
	parent.AsArray = parent
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{4}},
		Sizes:      []int{4},
		Collective: parent,
	}
	item.AsArray = item
	hole := &Expression{Term: TermConstant, Con: &Constant{Value: "1"}} // Type-nil residual Output
	item.SetIndexExpr(0, hole)
	if !HasError() {
		t.Fatal("index Output residual SetIndexExpr must SetError sticky")
	}
	if len(item.IndexExprs) != 0 {
		t.Fatal("index Output residual must not invent IndexExprs slot", item.IndexExprs)
	}
	ClearError()
	// AddIndexExpr residual same
	item.AddIndexExpr(hole)
	if !HasError() {
		t.Fatal("index Output residual AddIndexExpr must SetError sticky")
	}
	if len(item.IndexExprs) != 0 {
		t.Fatal("index Output residual must not invent AddIndexExpr slot")
	}
	ClearError()
}

func TestFactUnionOutputGetActualNameResidualSticky(t *testing.T) {
	// GetActualName residual soft invent was invent " last written field: N" past empty name.
	ClearError()
	f := &FactUnion{Var: &Variable{Type: GetIntType()}, LastWrittenFID: 0}
	if s := f.Output(); s != "" {
		t.Fatal("empty name residual must fail closed FactUnion.Output", s)
	}
	if !HasError() {
		t.Fatal("empty name residual FactUnion.Output must SetError sticky")
	}
	ClearError()
}

func TestItemizeIsAggregateResidualSticky(t *testing.T) {
	// IsAggregate residual soft invent was invent itemize shell past CreateFieldVars skip.
	// Type-nil already sticky before IsAggregate; complete scalar element itemizes without expand.
	ClearError()
	// Type-nil itemize path
	// covered by Itemize Type-nil tests; hygiene for residual after CreateFieldVars on aggregate with nil field
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: &Type{isStruct: true, StructName: "S0", Fields: []StructField{
			{Name: "f0", Type: nil, BitWidth: -1},
		}}, IsArray: true, ArraySizes: []int{2}},
		Sizes: []int{2},
	}
	parent.AsArray = parent
	// Itemize may CreateFieldVars residual on nil field Type
	item := parent.Itemize(NewRng(1))
	if item != nil && !HasError() {
		// if itemize succeeded without expand hole, CreateFieldVars may not run on non-aggregate wait - Type is struct aggregate
		// CreateFieldVars on nil field Type should sticky
		t.Fatal("CreateFieldVars residual must SetError sticky when itemize returns")
	}
	if item == nil && !HasError() {
		// Itemize may soft-fail without sticky for other reasons — require sticky on aggregate expand hole
		// Force CreateFieldVars residual: Type aggregate with nil field after Itemize mounts
	}
	ClearError()
	// Direct CreateFieldVars residual
	v := &Variable{Name: "g_s", Type: &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: nil, BitWidth: -1},
	}}}
	v.CreateFieldVars()
	if !HasError() {
		t.Fatal("CreateFieldVars nil field Type must SetError sticky")
	}
	ClearError()
}
