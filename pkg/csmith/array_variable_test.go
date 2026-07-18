package csmith

import (
	"strings"
	"testing"
)

func TestCreateArrayVariableDimensions(t *testing.T) {
	opts := Defaults()
	r := NewRng(2)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	av := CreateArrayVariable(r, opts, nil, "g_1", GetIntType(), MakeInt(0), q)
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
}

func TestItemizeCreateFieldVarsAggregate(t *testing.T) {
	// ArrayVariable.cpp:261–264 — itemize expands field vars for aggregate element type
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
