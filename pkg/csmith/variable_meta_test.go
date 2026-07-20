package csmith

import "testing"

func TestIsTmpVar(t *testing.T) {
	if !(&Variable{Name: "t_1"}).IsTmpVar() {
		t.Fatal("t_")
	}
	if (&Variable{Name: "g_1"}).IsTmpVar() {
		t.Fatal("g_")
	}
}

func TestIsValidVolatile(t *testing.T) {
	ClearError()
	// non-const always ok
	v := CreateVariableScalars("g_1", GetIntType(), true, false)
	if !v.IsValidVolatile() {
		t.Fatal("non-const")
	}
	// const null pointer invalid
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	p.Qfer.SetConst(true, 0)
	p.Init = MakeInt(0)
	if !p.IsConst() {
		t.Fatal("expected const")
	}
	if p.IsValidVolatile() {
		t.Fatal("const null ptr should be invalid volatile")
	}
	// const non-null ok
	p2 := CreateVariableScalars("g_p2", PointerTo(GetIntType()), true, false)
	p2.Qfer.SetConst(true, 0)
	p2.Init = MakeInt(1)
	if !p2.IsValidVolatile() {
		t.Fatal("const non-null")
	}
	ClearError()
	// Type-nil parent inside union-field path sticky invalid (no invent valid soft-skip)
	parent := &Variable{Name: "g_u"} // Type nil
	field := &Variable{Name: "g_u.f0", Type: GetIntType(), FieldVarOf: parent}
	if field.IsValidVolatile() {
		t.Fatal("Type-nil parent field IsValidVolatile must fail closed false")
	}
	if !HasError() {
		t.Fatal("Type-nil parent field IsValidVolatile must SetError sticky")
	}
	ClearError()
	// nested IsValidVolatile residual on container recurse soft invent was valid soft-skip.
	// Fair: sticky invalid via Type-nil container.
	ut := &Type{isUnion: true, Fields: []StructField{{Name: "f0", Type: GetIntType()}}}
	uv := &Variable{Name: "g_u2", Type: ut}
	// container itself incomplete Type-nil for nested recurse path via GetContainerUnion
	// IsInside residual already covered; also const Type-nil sticky invalid.
	cBroken := &Variable{Name: "g_c", Type: nil, Qfer: NewCVQualifiers([]bool{true}, []bool{false})}
	if cBroken.IsValidVolatile() {
		t.Fatal("Type-nil const IsValidVolatile must fail closed false")
	}
	if !HasError() {
		t.Fatal("Type-nil const IsValidVolatile must SetError sticky")
	}
	ClearError()
	_ = uv
}

func TestIsPackedAfterBitfield(t *testing.T) {
	ClearError()
	st := &Type{
		isStruct: true,
		Packed:   true,
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: 3},
			{Name: "f1", Type: GetIntType(), BitWidth: -1},
		},
	}
	parent := &Variable{Name: "g_s", Type: st}
	f0 := &Variable{Name: "g_s.f0", Type: GetIntType(), FieldVarOf: parent, IsBitfield: true}
	f1 := &Variable{Name: "g_s.f1", Type: GetIntType(), FieldVarOf: parent}
	parent.FieldVars = []*Variable{f0, f1}
	if f0.IsPackedAfterBitfield() {
		t.Fatal("first field not after bitfield")
	}
	if !f1.IsPackedAfterBitfield() {
		t.Fatal("f1 after bitfield in packed struct")
	}
	if HasError() {
		t.Fatal("complete IsPackedAfterBitfield must not sticky")
	}
	// incomplete FieldVars hole before f1: sticky packed-after (restrictive)
	ClearError()
	parent.FieldVars = []*Variable{f0, nil, f1}
	if !f1.IsPackedAfterBitfield() {
		t.Fatal("FieldVars hole must fail closed as packed-after-bitfield")
	}
	if !HasError() {
		t.Fatal("FieldVars hole IsPackedAfterBitfield must SetError sticky")
	}
	ClearError()
	// Type-nil parent sticky packed-after (restrictive — no invent not-packed soft-skip)
	parent.FieldVars = []*Variable{f0, f1}
	parent.Type = nil
	if !f1.IsPackedAfterBitfield() {
		t.Fatal("Type-nil parent must fail closed as packed-after-bitfield")
	}
	if !HasError() {
		t.Fatal("Type-nil parent IsPackedAfterBitfield must SetError sticky")
	}
	ClearError()
	// sibling Type-nil residual soft invent was soft-continue later siblings invent not-packed.
	// Fair: sticky packed-after true. Use non-bitfield field layout so walk hits Type-nil.
	stNoBF := &Type{
		isStruct: true,
		Packed:   true,
		Fields: []StructField{
			{Name: "fx", Type: GetIntType(), BitWidth: -1},
			{Name: "f1", Type: GetIntType(), BitWidth: -1},
		},
	}
	parent.Type = stNoBF
	sibHole := &Variable{Name: "g_s.fx", Type: nil, FieldVarOf: parent}
	parent.FieldVars = []*Variable{sibHole, f1}
	if !f1.IsPackedAfterBitfield() {
		t.Fatal("sibling Type-nil residual IsPackedAfterBitfield must fail closed true")
	}
	if !HasError() {
		t.Fatal("sibling Type-nil residual IsPackedAfterBitfield must SetError sticky")
	}
	ClearError()
	// nested HasBitfields residual on sibling: soft invent was soft-continue invent not-packed.
	// Fair: sticky packed-after true.
	innerHole := &Type{isStruct: true, Fields: []StructField{{Type: nil, BitWidth: -1}}}
	sibNest := &Variable{Name: "g_s.fn", Type: innerHole, FieldVarOf: parent}
	parent.FieldVars = []*Variable{sibNest, f1}
	if !f1.IsPackedAfterBitfield() {
		t.Fatal("HasBitfields residual IsPackedAfterBitfield must fail closed true")
	}
	if !HasError() {
		t.Fatal("HasBitfields residual IsPackedAfterBitfield must SetError sticky")
	}
	ClearError()
}

func TestGetSeqNum(t *testing.T) {
	// Variable.cpp:261–265 — assert '_' present
	ClearError()
	v := CreateVariableScalars("g_42", GetIntType(), false, false)
	if v == nil || v.GetSeqNum() != 42 {
		t.Fatal(v)
	}
	if HasError() {
		t.Fatal("complete GetSeqNum must not sticky")
	}
	ClearError()
	if (&Variable{Name: "badname"}).GetSeqNum() != -1 {
		t.Fatal("no underscore fail closed")
	}
	if !HasError() {
		t.Fatal("no underscore GetSeqNum must SetError sticky")
	}
	ClearError()
}

func TestGetCollectiveArrayField(t *testing.T) {
	// Variable.cpp:583–612 — field of itemized array maps to collective field
	ClearError()
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: &Type{isStruct: true, Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
		}}, IsArray: true, ArraySizes: []int{2}},
		Sizes: []int{2},
	}
	parent.AsArray = parent
	parent.CreateFieldVars()
	if len(parent.FieldVars) == 0 {
		t.Fatal("fields")
	}
	item := parent.ItemizeConstIndices([]int{1}, nil)
	if item == nil {
		t.Fatal("itemize")
	}
	item.CreateFieldVars()
	if len(item.FieldVars) == 0 {
		t.Fatal("item fields")
	}
	// itemized field collective should be parent field
	got := item.FieldVars[0].GetCollective()
	if got != parent.FieldVars[0] {
		t.Fatalf("want parent field, got %v", got)
	}
	// incomplete FieldVars on path fails closed sticky nil (no invent self as collective)
	ClearError()
	item.FieldVars[0].FieldVarOf.FieldVars = append(item.FieldVars[0].FieldVarOf.FieldVars, nil)
	// force hole on parent of field (item itself)
	item.FieldVars = append(item.FieldVars, nil)
	if item.FieldVars[0].GetCollective() != nil {
		t.Fatal("incomplete FieldVars must fail closed GetCollective nil, not invent self")
	}
	if !HasError() {
		t.Fatal("incomplete FieldVars GetCollective must SetError sticky")
	}
	ClearError()
	if ((*Variable)(nil)).GetCollective() != nil {
		t.Fatal("nil subject GetCollective must fail closed nil")
	}
	if !HasError() {
		t.Fatal("nil subject GetCollective must SetError sticky")
	}
	ClearError()
	// IsArray without AsArray ancestor soft invent was self-collective success
	// fair: sticky nil fail closed
	shell := &Variable{Name: "g_shell", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	fld := &Variable{Name: "g_shell.f0", Type: GetIntType(), FieldVarOf: shell}
	if fld.GetCollective() != nil {
		t.Fatal("IsArray without AsArray ancestor GetCollective must fail closed nil")
	}
	if !HasError() {
		t.Fatal("IsArray without AsArray ancestor GetCollective must SetError sticky")
	}
	ClearError()
	// bare IsArray without AsArray soft invent was return self as collective
	// fair: sticky nil fail closed
	if shell.GetCollective() != nil {
		t.Fatal("IsArray without AsArray GetCollective must fail closed nil, not invent self")
	}
	if !HasError() {
		t.Fatal("IsArray without AsArray bare GetCollective must SetError sticky")
	}
	ClearError()
}

func TestIsArrayField(t *testing.T) {
	// live AsArray parent
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}},
		Sizes:    []int{2},
	}
	av.AsArray = av
	field := &Variable{Name: "g_a[0].f0", Type: GetIntType(), FieldVarOf: &av.Variable}
	if !field.IsArrayField() {
		t.Fatal("array field")
	}
	// IsArray without AsArray soft invent was true without sticky
	// fair: still true restrictive + SetError sticky
	ClearError()
	shell := &Variable{Name: "g_b", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	field2 := &Variable{Name: "g_b[0].f0", Type: GetIntType(), FieldVarOf: shell}
	if !field2.IsArrayField() {
		t.Fatal("IsArray without AsArray parent must fail closed true restrictive")
	}
	if !HasError() {
		t.Fatal("IsArray without AsArray parent IsArrayField must SetError sticky")
	}
	ClearError()
	// nested IsArrayField residual soft invent was not-array-field soft-skip.
	// Fair: sticky true restrictive via nested IsArray without AsArray.
	mid := &Variable{Name: "g_m.f0", Type: GetIntType(), FieldVarOf: shell}
	deep := &Variable{Name: "g_m.f0.x", Type: GetIntType(), FieldVarOf: mid}
	if !deep.IsArrayField() {
		t.Fatal("nested IsArray residual IsArrayField must fail closed true")
	}
	if !HasError() {
		t.Fatal("nested IsArray residual IsArrayField must SetError sticky")
	}
	ClearError()
}

func TestIsPackedAggregateFieldVarNilSticky(t *testing.T) {
	ClearError()
	if (*Variable)(nil).IsPackedAggregateFieldVar() {
		t.Fatal("nil IsPackedAggregateFieldVar must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil IsPackedAggregateFieldVar must SetError sticky")
	}
	ClearError()
	// Type-nil ancestor sticky packed (restrictive — no invent not-packed soft-skip)
	parent := &Variable{Name: "g_s"} // Type nil
	field := &Variable{Name: "g_s.f0", Type: GetIntType(), FieldVarOf: parent}
	if !field.IsPackedAggregateFieldVar() {
		t.Fatal("Type-nil ancestor must fail closed as packed-aggregate field")
	}
	if !HasError() {
		t.Fatal("Type-nil ancestor IsPackedAggregateFieldVar must SetError sticky")
	}
	ClearError()
}
