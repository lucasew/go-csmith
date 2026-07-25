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
	ClearErrorSess(testAmbientSession)
	// non-const always ok
	v := CreateVariableScalars("g_1", GetIntType(), true, false)
	if !v.IsValidVolatile() {
		t.Fatal("non-const")
	}
	// const null pointer invalid
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	p.Qfer.SetConstSess(testAmbientSession, true, 0)
	p.Init = MakeInt(0)
	if !p.IsConst() {
		t.Fatal("expected const")
	}
	if p.IsValidVolatile() {
		t.Fatal("const null ptr should be invalid volatile")
	}
	// const non-null ok
	p2 := CreateVariableScalars("g_p2", PointerTo(GetIntType()), true, false)
	p2.Qfer.SetConstSess(testAmbientSession, true, 0)
	p2.Init = MakeInt(1)
	if !p2.IsValidVolatile() {
		t.Fatal("const non-null")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil parent inside union-field path sticky invalid (no invent valid soft-skip)
	parent := &Variable{Name: "g_u"} // Type nil
	field := &Variable{Name: "g_u.f0", Type: GetIntType(), FieldVarOf: parent}
	if field.IsValidVolatile() {
		t.Fatal("Type-nil parent field IsValidVolatile must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil parent field IsValidVolatile must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
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
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil const IsValidVolatile must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	_ = uv
}

func TestIsPackedAfterBitfield(t *testing.T) {
	ClearErrorSess(testAmbientSession)
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
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete IsPackedAfterBitfield must not sticky")
	}
	// incomplete FieldVars hole before f1: sticky packed-after (restrictive)
	ClearErrorSess(testAmbientSession)
	parent.FieldVars = []*Variable{f0, nil, f1}
	if !f1.IsPackedAfterBitfield() {
		t.Fatal("FieldVars hole must fail closed as packed-after-bitfield")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("FieldVars hole IsPackedAfterBitfield must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil parent sticky packed-after (restrictive — no invent not-packed soft-skip)
	parent.FieldVars = []*Variable{f0, f1}
	parent.Type = nil
	if !f1.IsPackedAfterBitfield() {
		t.Fatal("Type-nil parent must fail closed as packed-after-bitfield")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil parent IsPackedAfterBitfield must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
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
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("sibling Type-nil residual IsPackedAfterBitfield must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nested HasBitfields residual on sibling: soft invent was soft-continue invent not-packed.
	// Fair: sticky packed-after true.
	innerHole := &Type{isStruct: true, Fields: []StructField{{Type: nil, BitWidth: -1}}}
	sibNest := &Variable{Name: "g_s.fn", Type: innerHole, FieldVarOf: parent}
	parent.FieldVars = []*Variable{sibNest, f1}
	if !f1.IsPackedAfterBitfield() {
		t.Fatal("HasBitfields residual IsPackedAfterBitfield must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("HasBitfields residual IsPackedAfterBitfield must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGetSeqNum(t *testing.T) {
	// Variable.cpp:261–265 — assert '_' present
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalars("g_42", GetIntType(), false, false)
	if v == nil || v.GetSeqNum() != 42 {
		t.Fatal(v)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete GetSeqNum must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&Variable{Name: "badname"}).GetSeqNum() != -1 {
		t.Fatal("no underscore fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("no underscore GetSeqNum must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGetCollectiveTopLevelArray(t *testing.T) {
	// ArrayVariable.h:83–85 — get_collective returns collective ? collective : this.
	// Soft invent: is_array_field true for top-level arrays (Variable.cpp:270–276)
	// then base Variable::get_collective path assert(parent) — but C++ never runs that
	// for ArrayVariable* (virtual override). Go must take AsArray override first.
	// Broken path rejected every collective array in IsEligibleVar (seed-7 ok pool).
	ClearErrorSess(testAmbientSession)
	av := &ArrayVariable{
		Variable: Variable{Name: "g_16", Type: GetIntType(), IsArray: true, ArraySizes: []int{7}},
		Sizes:    []int{7},
	}
	av.AsArray = av
	// IsArrayField is true for top-level array (C++ returns isArray when no field_var_of)
	if !av.IsArrayField() {
		t.Fatal("top-level array IsArrayField must match C++ isArray when field_var_of null")
	}
	got := av.GetCollective()
	if got != &av.Variable {
		t.Fatalf("top-level collective GetCollective want self, got %v err=%v", got, HasErrorSess(testAmbientSession))
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("top-level collective GetCollective must not sticky", GetErrorSess(testAmbientSession))
	}
	// itemized member → collective parent
	item := av.ItemizeConstIndices([]int{3}, nil)
	if item == nil {
		t.Fatal("itemize")
	}
	got = item.GetCollective()
	if got != &av.Variable {
		t.Fatalf("itemized GetCollective want parent, got %v", got)
	}
	// IsEligibleVar must not reject collective array solely via GetCollective hole
	ClearErrorSess(testAmbientSession)
	if !IsEligibleVar(&av.Variable, 0, AccessRead, EmptyCGContext().WithSession(testAmbientSession)) {
		t.Fatal("collective array must be eligible under empty context", HasErrorSess(testAmbientSession))
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("eligible collective array must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGetCollectiveArrayField(t *testing.T) {
	// Variable.cpp:583–612 — field of itemized array maps to collective field
	ClearErrorSess(testAmbientSession)
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
	ClearErrorSess(testAmbientSession)
	item.FieldVars[0].FieldVarOf.FieldVars = append(item.FieldVars[0].FieldVarOf.FieldVars, nil)
	// force hole on parent of field (item itself)
	item.FieldVars = append(item.FieldVars, nil)
	if item.FieldVars[0].GetCollective() != nil {
		t.Fatal("incomplete FieldVars must fail closed GetCollective nil, not invent self")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete FieldVars GetCollective must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if ((*Variable)(nil)).GetCollective() != nil {
		t.Fatal("nil subject GetCollective must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil subject GetCollective must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray ancestor soft invent was self-collective success
	// fair: sticky nil fail closed
	shell := &Variable{Name: "g_shell", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	fld := &Variable{Name: "g_shell.f0", Type: GetIntType(), FieldVarOf: shell}
	if fld.GetCollective() != nil {
		t.Fatal("IsArray without AsArray ancestor GetCollective must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray ancestor GetCollective must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// bare IsArray without AsArray soft invent was return self as collective
	// fair: sticky nil fail closed
	if shell.GetCollective() != nil {
		t.Fatal("IsArray without AsArray GetCollective must fail closed nil, not invent self")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray bare GetCollective must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
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
	ClearErrorSess(testAmbientSession)
	shell := &Variable{Name: "g_b", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	field2 := &Variable{Name: "g_b[0].f0", Type: GetIntType(), FieldVarOf: shell}
	if !field2.IsArrayField() {
		t.Fatal("IsArray without AsArray parent must fail closed true restrictive")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray parent IsArrayField must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nested IsArrayField residual soft invent was not-array-field soft-skip.
	// Fair: sticky true restrictive via nested IsArray without AsArray.
	mid := &Variable{Name: "g_m.f0", Type: GetIntType(), FieldVarOf: shell}
	deep := &Variable{Name: "g_m.f0.x", Type: GetIntType(), FieldVarOf: mid}
	if !deep.IsArrayField() {
		t.Fatal("nested IsArray residual IsArrayField must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nested IsArray residual IsArrayField must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsPackedAggregateFieldVarNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*Variable)(nil).IsPackedAggregateFieldVar() {
		t.Fatal("nil IsPackedAggregateFieldVar must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsPackedAggregateFieldVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil ancestor sticky packed (restrictive — no invent not-packed soft-skip)
	parent := &Variable{Name: "g_s"} // Type nil
	field := &Variable{Name: "g_s.f0", Type: GetIntType(), FieldVarOf: parent}
	if !field.IsPackedAggregateFieldVar() {
		t.Fatal("Type-nil ancestor must fail closed as packed-aggregate field")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil ancestor IsPackedAggregateFieldVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsVirtualParentResidualSticky(t *testing.T) {
	// IsVirtual residual soft invent was invent soft not-virtual past IsArray without AsArray parent.
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray on self sticky
	shell := &Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	if shell.IsVirtual() {
		t.Fatal("IsArray without AsArray IsVirtual must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray IsVirtual must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// field of IsArray without AsArray parent residual recursive
	parent := &Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	field := &Variable{Name: "g_a.f0", Type: GetIntType(), FieldVarOf: parent}
	if field.IsVirtual() {
		t.Fatal("field of IsArray without AsArray parent must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("field of IsArray without AsArray parent IsVirtual must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGetCollectiveIsArrayFieldResidualSticky(t *testing.T) {
	// IsArrayField residual soft invent was invent soft-collective past FieldVarOf shell.
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray as FieldVarOf parent IsArrayField residual restrictive true
	parent := &Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	field := &Variable{Name: "g_a.f0", Type: GetIntType(), FieldVarOf: parent}
	// IsArrayField on field with parent IsArray without AsArray stickies residual true
	if field.GetCollective() != nil {
		// may incomplete nil
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArrayField residual GetCollective must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGetFieldIDIncompleteParentResidualSticky(t *testing.T) {
	// GetFieldID residual soft invent was invent field index past incomplete parent FieldVars.
	ClearErrorSess(testAmbientSession)
	parent := CreateVariableScalars("g_s", GetIntType(), false, false)
	field := &Variable{Name: "g_s.f0", Type: GetIntType(), FieldVarOf: parent}
	// incomplete parent FieldVars (hole)
	parent.FieldVars = []*Variable{nil, field}
	if field.GetFieldID() != -1 {
		t.Fatal("incomplete parent FieldVars GetFieldID must fail closed -1")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete parent FieldVars GetFieldID must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
